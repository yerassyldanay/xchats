package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/domain"
	"github.com/yerassyldanay/xchats/backend/internal/password"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

const sessionCookie = "xchats_session"

type ctxUserKey struct{}

// --- session middleware ---------------------------------------------------

func (s *Server) requireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, err := c.Cookie(sessionCookie)
		if err != nil || sid == "" {
			fail(c, http.StatusUnauthorized, ErrUnauthorized, "no session")
			return
		}
		u, err := s.store.UserForSession(ctx(c), sid)
		if err != nil {
			fail(c, http.StatusUnauthorized, ErrUnauthorized, "invalid session")
			return
		}
		c.Set("user", u)
		c.Set("sid", sid)
		c.Next()
	}
}

func currentUser(c *gin.Context) store.User {
	v, _ := c.Get("user")
	u, _ := v.(store.User)
	return u
}

// currentSessionID returns the raw session id requireSession already
// validated for this request — the active-organization lookup's key (Task
// 15: an org selection is per-BROWSER-SESSION, not per-user, so a user
// logged in on two devices can have each independently scoped).
func currentSessionID(c *gin.Context) string {
	v, _ := c.Get("sid")
	sid, _ := v.(string)
	return sid
}

// resolveOrg is orgOf's/mePayload's shared resolution core, parameterized
// directly on userID/sessionID rather than reading them off gin context —
// mePayload needs that distinction: handleLogin calls it for the
// just-authenticated user BEFORE requireSession has ever run for this
// request (there is no "current" context user yet at that point), so it
// must resolve for the user it was explicitly handed, not currentUser(c).
//
// Resolution order: the session's explicit active_organization_id (set by
// SetActiveOrganization, or by a verified MCP review handoff — plan Task 15),
// re-validated against CURRENT membership on every call so a stale session
// can never keep operating against an org the user was since removed from;
// falling back to OrgForUser's single-org deterministic default when no
// active organization is set (unchanged behavior for every user who has
// never switched orgs or landed via a handoff — including sessionID=="",
// which correctly resolves no active organization and falls through here).
func (s *Server) resolveOrg(c *gin.Context, userID uuid.UUID, sessionID string) (store.Organization, error) {
	if activeOrgID, ok, err := s.store.ActiveOrganizationForSession(ctx(c), sessionID); err == nil && ok {
		if inOrg, err := s.store.UserInOrg(ctx(c), userID, activeOrgID); err == nil && inOrg {
			if org, err := s.store.OrgByID(ctx(c), activeOrgID); err == nil {
				return org, nil
			}
		}
	}
	return s.store.OrgForUser(ctx(c), userID)
}

// orgOf resolves the current request's organization, failing the request if
// missing. It is the scoping root for the multi-account inbox, the accounts
// manager, and the Playground/KB editors.
func (s *Server) orgOf(c *gin.Context) (store.Organization, bool) {
	org, err := s.resolveOrg(c, currentUser(c).ID, currentSessionID(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "no organization")
		return store.Organization{}, false
	}
	return org, true
}

// RequireAdmin gates admin-only surfaces — Settings, organization rename,
// user creation, and role changes — behind the caller's live membership role
// in their current organization. Chained after requireSession on every route
// it guards; the role is re-derived from organization_users on each request,
// never cached on the session, so a demotion takes effect immediately even
// mid-session — the same freshness guarantee orgOf's own membership re-check
// already gives every org-scoped route.
func (s *Server) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		org, okOrg := s.orgOf(c)
		if !okOrg {
			return
		}
		role, err := s.store.MembershipRole(ctx(c), org.ID, currentUser(c).ID)
		if err != nil || role != "admin" {
			fail(c, http.StatusForbidden, ErrForbidden, "admin role required")
			return
		}
		c.Next()
	}
}

// --- handlers -------------------------------------------------------------

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "email and password required")
		return
	}
	u, err := s.store.UserByEmail(ctx(c), strings.TrimSpace(req.Email))
	if err != nil || !password.Verify(req.Password, u.PasswordHash) {
		fail(c, http.StatusUnauthorized, ErrUnauthorized, "invalid credentials")
		return
	}
	sid := newSessionID()
	ttl := time.Duration(s.cfg.System.SessionTTLHours) * time.Hour
	if err := s.store.CreateSession(ctx(c), sid, u.ID, ttl); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "session")
		return
	}
	s.setSessionCookie(c, sid, int(ttl.Seconds()))
	ok(c, s.mePayload(c, u))
}

func (s *Server) handleLogout(c *gin.Context) {
	if sid, err := c.Cookie(sessionCookie); err == nil && sid != "" {
		_ = s.store.DeleteSession(ctx(c), sid)
	}
	s.setSessionCookie(c, "", -1)
	ok(c, nil)
}

func (s *Server) handleMe(c *gin.Context) {
	ok(c, s.mePayload(c, currentUser(c)))
}

func (s *Server) mePayload(c *gin.Context, u store.User) gin.H {
	// u.ID, never currentUser(c).ID: handleLogin calls this for the
	// just-authenticated user before requireSession has ever populated gin
	// context for this request (see resolveOrg's doc comment).
	org, _ := s.resolveOrg(c, u.ID, currentSessionID(c))
	// role is scoped to THIS organization — a user's role can differ across
	// their memberships, so it is re-resolved here rather than trusted from
	// any cached User.Role (u itself carries none: UserForSession/UserByEmail
	// are not org-scoped). Left "" on error/no-membership, which the
	// frontend's isAdmin treats as non-admin — fail closed, never open.
	role, _ := s.store.MembershipRole(ctx(c), org.ID, u.ID)
	orgs, err := s.store.OrgsForUser(ctx(c), u.ID)
	if err != nil {
		orgs = nil
	}
	orgList := make([]gin.H, 0, len(orgs))
	for _, o := range orgs {
		orgList = append(orgList, gin.H{"id": o.ID, "name": o.Name})
	}
	return gin.H{
		"user":         gin.H{"id": u.ID, "email": u.Email, "name": u.DisplayName, "role": role},
		"organization": gin.H{"id": org.ID, "name": org.Name},
		// organizations is the full membership set, for the frontend's
		// active-organization switcher (Task 15) — omitted entirely from
		// this payload for a single-org user would be indistinguishable
		// from a load failure, so it is always present, even as a
		// one-element list.
		"organizations": orgList,
	}
}

type setActiveOrgReq struct {
	OrganizationID uuid.UUID `json:"organization_id"`
}

// handleSetActiveOrganization lets an operator explicitly switch which
// organization their session is scoped to (Task 15's frontend selector) —
// the same active_organization_id column a verified review-handoff redirect
// sets, just triggered directly instead of via a signed token. Membership is
// re-checked here (never trust the id merely because it was posted); orgOf
// re-checks it AGAIN on every subsequent request, so removal from an org
// takes effect immediately even mid-session.
func (s *Server) handleSetActiveOrganization(c *gin.Context) {
	var req setActiveOrgReq
	if err := c.ShouldBindJSON(&req); err != nil || req.OrganizationID == uuid.Nil {
		fail(c, http.StatusBadRequest, ErrValidation, "organization_id is required")
		return
	}
	u := currentUser(c)
	inOrg, err := s.store.UserInOrg(ctx(c), u.ID, req.OrganizationID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "membership check failed")
		return
	}
	if !inOrg {
		fail(c, http.StatusForbidden, ErrUnauthorized, "you are not a member of that organization")
		return
	}
	if err := s.store.SetActiveOrganization(ctx(c), currentSessionID(c), req.OrganizationID); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "failed to switch organization")
		return
	}
	ok(c, s.mePayload(c, u))
}

func (s *Server) handleGetOrg(c *gin.Context) {
	u := currentUser(c)
	org, err := s.store.OrgForUser(ctx(c), u.ID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "no organization")
		return
	}
	ok(c, gin.H{"id": org.ID, "name": org.Name, "auto_response_mode": org.RespondMode})
}

type updateOrgReq struct {
	Name string `json:"name"`
}

// handleUpdateOrg renames the current organization — the Team management
// UI's "org rename" action, and (per RequireAdmin, which gates this route)
// an admin-only one.
func (s *Server) handleUpdateOrg(c *gin.Context) {
	var req updateOrgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "name is required")
		return
	}
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	if err := s.store.RenameOrganization(ctx(c), org.ID, name); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"id": org.ID, "name": name, "auto_response_mode": org.RespondMode})
}

func (s *Server) handleListUsers(c *gin.Context) {
	org, proceed := s.orgOf(c)
	if !proceed {
		return
	}
	limit, offset, pageNum, pageSize := s.pageParams(c)
	users, total, err := s.store.ListUsersForOrg(ctx(c), org.ID, limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, gin.H{"id": u.ID, "email": u.Email, "name": u.DisplayName, "created_at": u.CreatedAt, "role": u.Role})
	}
	ok(c, page{Items: items, Page: pageNum, PageSize: pageSize, Total: total})
}

type createUserReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Server) handleCreateUser(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "email required")
		return
	}
	if len(req.Password) < s.cfg.System.MinPasswordLen {
		fail(c, http.StatusBadRequest, ErrValidation, fmt.Sprintf("password must be >= %d chars", s.cfg.System.MinPasswordLen))
		return
	}
	org, err := s.store.OrgForUser(ctx(c), currentUser(c).ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "no org")
		return
	}
	hash, err := password.Hash(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "hash")
		return
	}
	// New team members always start as "member" — promote them afterward via
	// PUT /users/:id/role (Team management UI's role toggle).
	u, err := s.store.CreateUser(ctx(c), org.ID, strings.TrimSpace(req.Email), hash, req.Name, "member")
	if err != nil {
		if isUniqueViolation(err) {
			fail(c, http.StatusConflict, ErrConflict, "email already exists")
			return
		}
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	created(c, gin.H{"id": u.ID, "email": u.Email, "name": u.DisplayName, "role": u.Role})
}

type updateUserRoleReq struct {
	Role string `json:"role"`
}

// handleUpdateUserRole is the Team management UI's role toggle. Gated by
// RequireAdmin like the rest of this file's admin-only routes; the store
// layer additionally refuses to demote an organization's last admin
// (domain.ErrLastAdmin), so this can 409 even for an authorized admin caller.
func (s *Server) handleUpdateUserRole(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req updateUserRoleReq
	if err := c.ShouldBindJSON(&req); err != nil || (req.Role != "admin" && req.Role != "member") {
		fail(c, http.StatusBadRequest, ErrValidation, `role must be "admin" or "member"`)
		return
	}
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	if err := s.store.SetMembershipRole(ctx(c), org.ID, id, req.Role); err != nil {
		switch {
		case errors.Is(err, domain.ErrLastAdmin):
			fail(c, http.StatusConflict, ErrConflict, "cannot change the role of an organization's last admin")
		case errors.Is(err, store.ErrNotFound):
			fail(c, http.StatusNotFound, ErrNotFound, "not a member of this organization")
		default:
			fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		}
		return
	}
	ok(c, gin.H{"id": id, "role": req.Role})
}

func (s *Server) setSessionCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.requestIsSecure(c),
		SameSite: http.SameSiteLaxMode,
	})
}

// requestIsSecure reports whether this request's origin is HTTPS: the
// static SecureCookies config flag (an operator's permanent setting), a
// direct TLS connection, or X-Forwarded-Proto: https — how a TLS-terminating
// front door (a reverse proxy, or the embedded ngrok tunnel's own edge —
// internal/tunnel, plan/DECISIONS.md's tunnel-exposure amendment) tells the
// app it did. Unlike ClientIP's TrustedProxies-gated trust, this header is
// honored unconditionally: the downside of a spoofed one is low (a Secure
// cookie the browser then withholds on a later plain-HTTP request, breaking
// that one session) and never a leak, so no separate trust gate is needed.
func (s *Server) requestIsSecure(c *gin.Context) bool {
	if s.cfg.Server.SecureCookies || c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func newSessionID() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// isUniqueViolation reports whether err is the persistence layer's
// "a unique constraint rejected this write" signal.
//
// This used to string-match "23505", PostgreSQL's SQLSTATE for a unique
// violation as it appeared inside a pgx error message. That made an HTTP
// handler depend on which database engine happened to be underneath it, and it
// silently stopped matching the moment one wasn't PostgreSQL — turning a 409
// into a 500 with no test to catch it. internal/store already translates driver
// errors into the domain vocabulary at its own exported boundary (see
// store.CreateUser), so the engine-neutral sentinel is what to compare against.
func isUniqueViolation(err error) bool {
	return errors.Is(err, domain.ErrDuplicate)
}
