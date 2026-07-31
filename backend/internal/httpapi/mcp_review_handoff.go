// mcp_review_handoff.go implements Task 15's organization-bound review
// handoff: the KB Manager widget's "Review and publish in Xchats" link is
// minted per-tool-call by mcpserver.Deps.SignReviewHandoff (a one-time,
// organization-bound signed URL, never the MCP access token itself) and
// redeemed here, landing a multi-organization user on the SAME organization
// the calling tool operated on rather than whichever org OrgForUser's
// single-org default would otherwise pick.
package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
)

// handleReviewHandoff validates the token (signature, issuer, audience,
// expiry), rejects a replay (the mcp_access_token_denylist jti-tracking
// table an early-revoked access token also uses — reused here as "already
// consumed" rather than "explicitly revoked", the same mechanic either way:
// once a jti appears in that table, the token is dead), rejects a token
// whose Subject does not match the CURRENTLY logged-in browser session, and
// re-checks CURRENT organization membership (never trusts the token's own
// claim that membership held at mint time). Only then does it set the
// session's active organization and redirect to a clean /playground URL.
func (s *Server) handleReviewHandoff(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		s.renderOAuthError(c, "Коннектор MCP не настроен на этом сервере.")
		return
	}
	token := c.Query("token")
	if token == "" {
		s.renderOAuthError(c, "Отсутствует токен перехода.")
		return
	}
	user, sid, ok := s.oauthSessionUser(c)
	if !ok {
		s.renderOAuthError(c, "Войдите в Xchats, затем откройте эту ссылку ещё раз.")
		return
	}

	signer := mcpauth.NewReviewHandoffSigner(s.mcpAuth.Key, s.mcpIssuer(), time.Duration(s.cfg.MCPReviewHandoffTTLSeconds)*time.Second)
	claims, err := signer.Verify(token)
	if err != nil {
		s.renderOAuthError(c, "Ссылка недействительна или истекла. Запросите новую ссылку из ИИ-помощника.")
		return
	}
	if claims.Subject != user.ID.String() {
		s.renderOAuthError(c, "Эта ссылка предназначена для другого пользователя.")
		return
	}
	orgID, err := uuid.Parse(claims.OrganizationID)
	if err != nil {
		s.renderOAuthError(c, "Недействительная организация в ссылке.")
		return
	}
	denied, err := s.mcpAuth.Store.IsJTIDenied(ctx(c), claims.JTI)
	if err != nil {
		s.renderOAuthError(c, "Не удалось проверить ссылку.")
		return
	}
	if denied {
		s.renderOAuthError(c, "Эта ссылка уже была использована. Запросите новую ссылку из ИИ-помощника.")
		return
	}
	inOrg, err := s.store.UserInOrg(ctx(c), user.ID, orgID)
	if err != nil || !inOrg {
		s.renderOAuthError(c, "Вы больше не состоите в этой организации.")
		return
	}

	// Consume FIRST: from this instant the token can never be redeemed
	// again, even if SetActiveOrganization below fails — the alternative
	// (set-then-consume) would leave a window where a concurrent replay of
	// the same token could still succeed.
	if err := s.mcpAuth.Store.DenylistJTI(ctx(c), claims.JTI, time.Unix(claims.ExpiresAt, 0)); err != nil {
		s.renderOAuthError(c, "Не удалось завершить переход.")
		return
	}
	if err := s.store.SetActiveOrganization(ctx(c), sid, orgID); err != nil {
		s.renderOAuthError(c, "Не удалось переключить организацию.")
		return
	}
	c.Redirect(http.StatusFound, s.cfg.MCPResolvedFrontendBaseURL()+"/playground")
}
