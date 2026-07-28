package responsestore

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/response"
)

// cacheTTL is the backstop refresh window for an edit that bypasses the /kb/*
// API entirely (a direct SQL write) — every /kb/* write instead invalidates
// synchronously (httpapi.kbLiveChanged -> Invalidate), so in practice a UI
// edit is visible on the NEXT Load, well inside this TTL.
const cacheTTL = 60 * time.Second

type cacheEntry struct {
	kb      *aiprompt.KB
	builtAt time.Time
}

// CachedKBRepo wraps a response.KnowledgeBaseRepository with a per-org, in-
// memory cache: ONE shared build of the prompt-facing KB that both the
// response engine's hot path (every customer reply) and GET /kb/prompt (the
// /knowledge-base "Промпт" tab) read from, instead of each re-querying
// Postgres and rebuilding the same catalog independently. This is what makes
// the Промпт tab an honest preview of what the AI actually reads: it is
// reading the identical cached *aiprompt.KB the reply path would.
type CachedKBRepo struct {
	Inner response.KnowledgeBaseRepository

	mu      sync.Mutex
	entries map[string]cacheEntry
}

// NewCachedKBRepo builds a ready-to-use cache wrapping inner.
func NewCachedKBRepo(inner response.KnowledgeBaseRepository) *CachedKBRepo {
	return &CachedKBRepo{Inner: inner, entries: make(map[string]cacheEntry)}
}

// Load returns organizationID's cached *aiprompt.KB when fresh (built within
// the last cacheTTL and not explicitly Invalidate-d since), else loads
// through Inner and caches the result. A failed load is never cached — the
// next call retries against Inner rather than pinning the error for cacheTTL.
func (r *CachedKBRepo) Load(ctx context.Context, organizationID string) (*aiprompt.KB, error) {
	r.mu.Lock()
	e, ok := r.entries[organizationID]
	r.mu.Unlock()
	if ok && time.Since(e.builtAt) < cacheTTL {
		return e.kb, nil
	}

	kb, err := r.Inner.Load(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.entries[organizationID] = cacheEntry{kb: kb, builtAt: time.Now()}
	r.mu.Unlock()
	return kb, nil
}

// Invalidate drops orgID's cached entry, if any, so the next Load re-reads
// Postgres instead of serving a stale build. Safe to call for an org with no
// cached entry (a plain no-op).
func (r *CachedKBRepo) Invalidate(orgID uuid.UUID) {
	r.mu.Lock()
	delete(r.entries, orgID.String())
	r.mu.Unlock()
}
