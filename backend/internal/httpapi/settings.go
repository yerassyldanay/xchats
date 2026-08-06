package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/providerhealth"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/internal/version"
)

// --- GET /settings ----------------------------------------------------

// handleGetSettings returns the full Settings object: LLM defaults, ngrok's
// region/domain, and the two onboarding flags. Provider-specific credential
// state (configured/source) is NOT here — see handleListIntegrations,
// which merges this same Settings.Providers data with live credential-store
// lookups the way a provider CARD in the UI needs, one call each.
func (s *Server) handleGetSettings(c *gin.Context) {
	if s.settings == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "settings store is unavailable")
		return
	}
	st, err := s.settings.Load()
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, st)
}

// --- GET /settings/integrations ----------------------------------------

type fieldSummary struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type integrationSummary struct {
	ID            string         `json:"id"`
	DisplayName   string         `json:"display_name"`
	CredentialURL string         `json:"credential_url"`
	DocsURL       string         `json:"docs_url"`
	HasBaseURL    bool           `json:"has_base_url"`
	HasModel      bool           `json:"has_model"`
	Fields        []fieldSummary `json:"fields"`
	Validatable   bool           `json:"validatable"`

	// Configured/Source come from the live credential store (nil-tolerant:
	// both stay at their zero value when s.credentials is nil).
	Configured bool   `json:"configured"`
	Source     string `json:"source,omitempty"` // "env" | "keyring" | "file" — omitted when not configured

	// The rest comes from Settings.Providers[id] — never populated when
	// s.settings is nil.
	BaseURL        string     `json:"base_url,omitempty"`
	DefaultModel   string     `json:"default_model,omitempty"`
	Disabled       bool       `json:"disabled"`
	LastVerifiedAt *time.Time `json:"last_verified_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
}

// handleListIntegrations lists every credential-backed integration's static
// metadata (internal/credentials.Providers) merged with its live
// configured/source state and its non-secret Settings.
func (s *Server) handleListIntegrations(c *gin.Context) {
	var st settings.Settings
	if s.settings != nil {
		if loaded, err := s.settings.Load(); err == nil {
			st = loaded
		}
	}

	providers := credentials.Providers()
	out := make([]integrationSummary, 0, len(providers))
	for _, p := range providers {
		sum := integrationSummary{
			ID: p.ID, DisplayName: p.DisplayName, CredentialURL: p.CredentialURL, DocsURL: p.DocsURL,
			HasBaseURL: p.HasBaseURL, HasModel: p.HasModel, Validatable: p.Validate != nil,
		}
		for _, f := range p.Fields {
			sum.Fields = append(sum.Fields, fieldSummary{Key: string(f.Key), Label: f.Label})
		}

		if s.credentials != nil {
			sum.Configured = s.providerConfigured(ctx(c), p)
			if sum.Configured {
				if src, err := s.credentials.Source(ctx(c), p.Fields[0].Key); err == nil {
					sum.Source = src
				}
			}
		}

		if ps, ok := st.Providers[p.ID]; ok {
			sum.BaseURL = ps.BaseURL
			sum.DefaultModel = ps.DefaultModel
			sum.Disabled = ps.Disabled
			sum.LastVerifiedAt = ps.LastVerifiedAt
			sum.LastError = ps.LastError
		}
		out = append(out, sum)
	}
	ok(c, gin.H{"credential_store_available": s.credentials != nil, "providers": out})
}

// providerConfigured reports whether EVERY one of p's credential fields
// resolves to a value — a provider like langfuse needs both its public and
// secret key to be usable at all, so a partially-configured provider is
// reported as not configured rather than misleadingly "half done."
func (s *Server) providerConfigured(ctx context.Context, p credentials.Provider) bool {
	for _, f := range p.Fields {
		if _, err := s.credentials.Get(ctx, f.Key); err != nil {
			return false
		}
	}
	return true
}

// integrationProvider resolves the :provider path param, failing the
// request with 404 if it names no known provider.
func (s *Server) integrationProvider(c *gin.Context) (credentials.Provider, bool) {
	id := c.Param("provider")
	p, ok := credentials.ProviderByID(id)
	if !ok {
		fail(c, http.StatusNotFound, ErrNotFound, fmt.Sprintf("unknown provider %q", id))
		return credentials.Provider{}, false
	}
	return p, true
}

// baseURLValueFor adds "<id>.base_url" to values when one is configured in
// Settings. HasBaseURL controls whether the UI exposes that setting, but the
// hidden override remains a useful HTTP-test seam for fixed-origin providers
// such as ngrok. "" (unset) is left out; validators use their public default.
func (s *Server) baseURLValueFor(p credentials.Provider, values map[credentials.Key]string) {
	if s.settings == nil {
		return
	}
	cur, err := s.settings.Load()
	if err != nil {
		return
	}
	if base := cur.Providers[p.ID].BaseURL; base != "" {
		values[credentials.Key(p.ID+".base_url")] = base
	}
}

// verifyProviderCredential runs p's live check against values (if it has
// one) and records the result in Settings.Providers[p.ID]
// (LastVerifiedAt/LastError) — used by both the save and test handlers
// below. A provider with no Validate is reported as
// credentials.ErrValidationUnavailable: "no check exists" and "the check
// couldn't complete" both mean the same thing to a caller — neither
// confirms the credential is good or bad.
func (s *Server) verifyProviderCredential(ctx context.Context, p credentials.Provider, values map[credentials.Key]string) error {
	if p.Validate == nil {
		return credentials.ErrValidationUnavailable
	}
	verifyErr := p.Validate(ctx, values)
	if s.settings != nil {
		now := time.Now().UTC()
		_, _ = s.settings.Update(func(st *settings.Settings) {
			ps := st.Providers[p.ID]
			if verifyErr == nil {
				ps.LastVerifiedAt = &now
				ps.LastError = ""
			} else {
				ps.LastError = verifyErr.Error()
			}
			st.Providers[p.ID] = ps
		})
	}
	return verifyErr
}

// --- PUT /settings/integrations/:provider (non-secret settings) --------

type updateProviderSettingsReq struct {
	BaseURL      string `json:"base_url"`
	DefaultModel string `json:"default_model"`
	Disabled     bool   `json:"disabled"`
}

// handleUpdateIntegrationSettings replaces a provider's non-secret
// settings (base URL / default model / disabled) as one unit — changing
// any of them resets LastVerifiedAt/LastError: a prior verification is not
// meaningful evidence about a since-changed endpoint.
func (s *Server) handleUpdateIntegrationSettings(c *gin.Context) {
	provider, okProvider := s.integrationProvider(c)
	if !okProvider {
		return
	}
	if s.settings == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "settings store is unavailable")
		return
	}
	var req updateProviderSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	updated, err := s.settings.Update(func(st *settings.Settings) {
		st.Providers[provider.ID] = settings.ProviderSettings{
			BaseURL:      strings.TrimSpace(req.BaseURL),
			DefaultModel: strings.TrimSpace(req.DefaultModel),
			Disabled:     req.Disabled,
		}
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	if s.llmRefresh != nil {
		s.llmRefresh()
	}
	ps := updated.Providers[provider.ID]
	ok(c, gin.H{"id": provider.ID, "base_url": ps.BaseURL, "default_model": ps.DefaultModel, "disabled": ps.Disabled})
}

// --- PUT/DELETE /settings/integrations/:provider/credential ------------

type saveCredentialReq struct {
	Values map[string]string `json:"values"`
	// Force saves an unverifiable credential anyway (the check itself
	// could not complete — network trouble, a provider outage — never a
	// confirmed rejection; ErrInvalidCredential can never be forced past).
	Force bool `json:"force"`
}

// handleSaveIntegrationCredential validates then saves a provider's
// credential fields: a confirmed-invalid credential (ErrInvalidCredential)
// is NEVER saved, regardless of Force — "invalid never becomes configured."
// An unverifiable one (ErrValidationUnavailable, or a provider with no
// Validate function) is saved only when Force is set, or when there is no
// check to run in the first place.
func (s *Server) handleSaveIntegrationCredential(c *gin.Context) {
	provider, okProvider := s.integrationProvider(c)
	if !okProvider {
		return
	}
	if s.credentials == nil {
		fail(c, http.StatusServiceUnavailable, ErrCredentialStore, "no credential store is available")
		return
	}
	var req saveCredentialReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}

	values := make(map[credentials.Key]string, len(provider.Fields))
	for _, f := range provider.Fields {
		v := strings.TrimSpace(req.Values[string(f.Key)])
		if v == "" {
			// Secret fields are never echoed to the UI. Preserve an already
			// stored sibling when adding/replacing one field of a multi-field
			// credential (notably adding ngrok.api_key to an existing token).
			existing, err := s.credentials.Get(ctx(c), f.Key)
			if err != nil {
				fail(c, http.StatusBadRequest, ErrValidation, fmt.Sprintf("missing value for %q", f.Key))
				return
			}
			v = existing
		}
		values[f.Key] = v
	}

	verified := false
	if provider.Validate != nil {
		s.baseURLValueFor(provider, values)
		switch verifyErr := s.verifyProviderCredential(ctx(c), provider, values); {
		case verifyErr == nil:
			verified = true
		case errors.Is(verifyErr, credentials.ErrInvalidCredential):
			fail(c, http.StatusUnprocessableEntity, ErrCredentialInvalid, "credential was rejected by the provider")
			return
		case errors.Is(verifyErr, credentials.ErrValidationUnavailable):
			if !req.Force {
				fail(c, http.StatusConflict, ErrCredentialUnverified,
					"could not verify the credential; resubmit with force=true to save it anyway")
				return
			}
		default:
			fail(c, http.StatusInternalServerError, ErrInternal, verifyErr.Error())
			return
		}
	}

	for _, f := range provider.Fields {
		if err := s.credentials.Set(ctx(c), f.Key, values[f.Key]); err != nil {
			fail(c, http.StatusInternalServerError, ErrCredentialStore, err.Error())
			return
		}
	}
	if s.llmRefresh != nil {
		s.llmRefresh()
	}
	ok(c, gin.H{"id": provider.ID, "verified": verified})
}

// handleDeleteIntegrationCredential removes a provider's stored credential.
// A field currently sourced from the environment (Chain.Source == "env")
// refuses with a clear message instead of silently no-opping: deleting the
// stored value would have no visible effect, since the env overlay would
// still win every read.
func (s *Server) handleDeleteIntegrationCredential(c *gin.Context) {
	provider, okProvider := s.integrationProvider(c)
	if !okProvider {
		return
	}
	if s.credentials == nil {
		fail(c, http.StatusServiceUnavailable, ErrCredentialStore, "no credential store is available")
		return
	}
	for _, f := range provider.Fields {
		if src, err := s.credentials.Source(ctx(c), f.Key); err == nil && src == "env" {
			fail(c, http.StatusConflict, ErrCredentialStore,
				fmt.Sprintf("%s is managed by the environment and cannot be deleted from the Settings UI", f.Key))
			return
		}
	}
	for _, f := range provider.Fields {
		if err := s.credentials.Delete(ctx(c), f.Key); err != nil && !errors.Is(err, credentials.ErrNotFound) {
			fail(c, http.StatusInternalServerError, ErrCredentialStore, err.Error())
			return
		}
	}
	if s.settings != nil {
		_, _ = s.settings.Update(func(st *settings.Settings) {
			delete(st.Providers, provider.ID)
		})
	}
	if s.llmRefresh != nil {
		s.llmRefresh()
	}
	ok(c, gin.H{"id": provider.ID})
}

// --- POST /settings/integrations/:provider/test -------------------------

// handleTestIntegrationCredential re-validates a provider's already-STORED
// credential (as opposed to save's not-yet-saved values) — the Settings
// UI's "test connection" button on an existing integration card.
func (s *Server) handleTestIntegrationCredential(c *gin.Context) {
	provider, okProvider := s.integrationProvider(c)
	if !okProvider {
		return
	}
	// A static fact about the provider, independent of what (if anything)
	// is configured yet — checked before the stored-credential lookup below
	// so a provider with no check reports that fact instead of a misleading
	// "not configured" response.
	if provider.Validate == nil {
		fail(c, http.StatusBadRequest, ErrValidation, "this integration has no credential check")
		return
	}
	if s.credentials == nil {
		fail(c, http.StatusServiceUnavailable, ErrCredentialStore, "no credential store is available")
		return
	}

	values := make(map[credentials.Key]string, len(provider.Fields))
	for _, f := range provider.Fields {
		v, err := s.credentials.Get(ctx(c), f.Key)
		if err != nil {
			fail(c, http.StatusNotFound, ErrNotFound, fmt.Sprintf("%s is not configured", f.Key))
			return
		}
		values[f.Key] = v
	}
	s.baseURLValueFor(provider, values)

	switch verifyErr := s.verifyProviderCredential(ctx(c), provider, values); {
	case verifyErr == nil:
		ok(c, gin.H{"id": provider.ID, "verified": true})
	case errors.Is(verifyErr, credentials.ErrInvalidCredential):
		fail(c, http.StatusUnprocessableEntity, ErrCredentialInvalid, "credential was rejected by the provider")
	case errors.Is(verifyErr, credentials.ErrValidationUnavailable):
		fail(c, http.StatusServiceUnavailable, ErrCredentialUnverified, "could not verify the credential right now")
	default:
		fail(c, http.StatusInternalServerError, ErrInternal, verifyErr.Error())
	}
}

// --- PUT /settings/llm ---------------------------------------------------

type updateLLMSettingsReq struct {
	DefaultProvider string  `json:"default_provider"`
	DefaultModel    string  `json:"default_model"`
	VisionModel     string  `json:"vision_model"`
	MaxTokens       int     `json:"max_tokens"`
	Temperature     float64 `json:"temperature"`
	TimeoutSeconds  int     `json:"timeout_seconds"`
	Retry           bool    `json:"retry"`
}

func (s *Server) handleUpdateLLMSettings(c *gin.Context) {
	if s.settings == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "settings store is unavailable")
		return
	}
	var req updateLLMSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil || req.DefaultProvider == "" || req.DefaultModel == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "default_provider and default_model are required")
		return
	}
	if _, known := credentials.ProviderByID(req.DefaultProvider); !known {
		fail(c, http.StatusBadRequest, ErrValidation, fmt.Sprintf("unknown provider %q", req.DefaultProvider))
		return
	}
	if req.MaxTokens <= 0 {
		fail(c, http.StatusBadRequest, ErrValidation, "max_tokens must be positive")
		return
	}
	if req.Temperature < 0 || req.Temperature > 2 {
		fail(c, http.StatusBadRequest, ErrValidation, "temperature must be between 0 and 2")
		return
	}
	if req.TimeoutSeconds <= 0 {
		fail(c, http.StatusBadRequest, ErrValidation, "timeout_seconds must be positive")
		return
	}

	updated, err := s.settings.Update(func(st *settings.Settings) {
		st.LLM = settings.LLMSettings{
			DefaultProvider: req.DefaultProvider, DefaultModel: req.DefaultModel, VisionModel: req.VisionModel,
			MaxTokens: req.MaxTokens, Temperature: req.Temperature, TimeoutSeconds: req.TimeoutSeconds, Retry: req.Retry,
		}
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	if s.llmRefresh != nil {
		s.llmRefresh()
	}
	ok(c, updated.LLM)
}

// --- PUT /settings/credential-storage ------------------------------------

type updateCredentialStorageReq struct {
	CredentialFileFallbackAccepted bool `json:"credential_file_fallback_accepted"`
}

// handleUpdateCredentialStorage records that an operator explicitly
// acknowledged the file-backed credential store's weaker guarantee versus
// an OS keychain (see credentials.Open's own doc comment) — shown once by
// the Settings UI, not on every save.
func (s *Server) handleUpdateCredentialStorage(c *gin.Context) {
	if s.settings == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "settings store is unavailable")
		return
	}
	var req updateCredentialStorageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	updated, err := s.settings.Update(func(st *settings.Settings) {
		st.CredentialFileFallbackAccepted = req.CredentialFileFallbackAccepted
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"credential_file_fallback_accepted": updated.CredentialFileFallbackAccepted})
}

// --- POST /settings/setup-complete ---------------------------------------

// handleSetupComplete is the first-run wizard's "don't show this again."
func (s *Server) handleSetupComplete(c *gin.Context) {
	if s.settings == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "settings store is unavailable")
		return
	}
	updated, err := s.settings.Update(func(st *settings.Settings) {
		st.SetupCompleted = true
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"setup_completed": updated.SetupCompleted})
}

// --- PUT /settings/ngrok ---------------------------------------------------

type updateNgrokSettingsReq struct {
	Region string `json:"region"`
	Domain string `json:"domain"`
}

// handleUpdateNgrokSettings sets the embedded tunnel's region/reserved-domain
// preference. Takes effect on the next Start (tunnel.Manager.tunnelOptions
// reads Settings fresh every time it connects) — never a restart of an
// already-running tunnel, since a live ngrok session isn't reconfigurable
// in place.
func (s *Server) handleUpdateNgrokSettings(c *gin.Context) {
	if s.settings == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "settings store is unavailable")
		return
	}
	var req updateNgrokSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	updated, err := s.settings.Update(func(st *settings.Settings) {
		st.Ngrok = settings.NgrokSettings{
			Region: strings.TrimSpace(req.Region),
			Domain: strings.TrimSpace(req.Domain),
		}
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, updated.Ngrok)
}

// --- GET /settings/backup/download -----------------------------------------

// handleDownloadBackup streams a complete disaster-recovery archive — see
// store.BackupZip/dbops.BackupZip's own doc comments for exactly what it
// contains (a DB snapshot, media, non-secret settings) and what it
// deliberately never does (the credential store). Errors are only reported
// as a normal envelope if they happen BEFORE any bytes are written — once
// streaming starts, headers are already committed 200 OK, so a mid-stream
// failure can only be logged, not turned into an error response.
func (s *Server) handleDownloadBackup(c *gin.Context) {
	if s.store == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "the database is unavailable")
		return
	}
	var settingsJSON []byte
	if s.settings != nil {
		if st, err := s.settings.Load(); err == nil {
			settingsJSON, _ = json.Marshal(st)
		}
	}

	filename := fmt.Sprintf("xchats-backup-%s.zip", time.Now().UTC().Format("20060102-150405"))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Status(http.StatusOK)

	blobDir := ""
	if s.cfg != nil {
		blobDir = s.cfg.Storage.BlobDir
	}
	if _, err := s.store.BackupZip(ctx(c), blobDir, settingsJSON, c.Writer); err != nil {
		s.log.Error("backup zip failed mid-stream", "err", err)
	}
}

// --- GET /settings/provider-health -----------------------------------------

// handleProviderHealth reports every LLM provider's live, in-production
// health (internal/providerhealth) — the on-demand hydration for a client
// that opens Settings (or just connects to /realtime) after a provider
// already flipped unhealthy, complementing the "integration.status_changed"
// event Track 2K broadcasts live on every transition.
func (s *Server) handleProviderHealth(c *gin.Context) {
	providers := []providerhealth.Status{}
	if s.providerHealth != nil {
		if snap := s.providerHealth.Snapshot(); snap != nil {
			providers = snap
		}
	}
	ok(c, gin.H{"providers": providers})
}

// --- GET /settings/update-check ---------------------------------------------

// handleUpdateCheck reports whether a newer xchats release exists
// (internal/updatecheck) — cached well past this one request, so this is
// cheap to call on every Settings page load.
func (s *Server) handleUpdateCheck(c *gin.Context) {
	if s.updateChecker == nil {
		ok(c, gin.H{"current_version": version.Version, "update_available": false})
		return
	}
	ok(c, s.updateChecker.Check(ctx(c)))
}

// --- tunnel ---------------------------------------------------------------

func (s *Server) handleGetTunnelStatus(c *gin.Context) {
	if s.tunnel == nil {
		fail(c, http.StatusServiceUnavailable, ErrTunnelUnavailable, "the tunnel feature is not available in this deployment")
		return
	}
	ok(c, s.tunnel.Status())
}

func (s *Server) handleStartTunnel(c *gin.Context) {
	if s.tunnel == nil {
		fail(c, http.StatusServiceUnavailable, ErrTunnelUnavailable, "the tunnel feature is not available in this deployment")
		return
	}
	if err := s.tunnel.Start(ctx(c)); err != nil {
		failWithPayload(c, http.StatusBadGateway, ErrTunnelUnavailable, err.Error(), s.tunnel.Status())
		return
	}
	ok(c, s.tunnel.Status())
}

func (s *Server) handleStopTunnel(c *gin.Context) {
	if s.tunnel == nil {
		fail(c, http.StatusServiceUnavailable, ErrTunnelUnavailable, "the tunnel feature is not available in this deployment")
		return
	}
	if err := s.tunnel.Stop(ctx(c)); err != nil {
		failWithPayload(c, http.StatusInternalServerError, ErrTunnelUnavailable, err.Error(), s.tunnel.Status())
		return
	}
	ok(c, s.tunnel.Status())
}
