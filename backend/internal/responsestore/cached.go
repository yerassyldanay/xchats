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
	gen     uint64
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
	// gens tracks the current invalidation generation per org so a Load that
	// started before an Invalidate can't clobber it: a concurrent write's
	// Invalidate() during another goroutine's in-flight DB read must not have
	// that stale read win the race and get cached anyway.
	gens map[string]uint64
}

// NewCachedKBRepo builds a ready-to-use cache wrapping inner.
func NewCachedKBRepo(inner response.KnowledgeBaseRepository) *CachedKBRepo {
	return &CachedKBRepo{
		Inner:   inner,
		entries: make(map[string]cacheEntry),
		gens:    make(map[string]uint64),
	}
}

// Load returns organizationID's cached *aiprompt.KB when fresh (built within
// the last cacheTTL and not explicitly Invalidate-d since), else loads
// through Inner and caches the result. A failed load is never cached — the
// next call retries against Inner rather than pinning the error for cacheTTL.
func (r *CachedKBRepo) Load(ctx context.Context, organizationID string) (*aiprompt.KB, error) {
	r.mu.Lock()
	e, ok := r.entries[organizationID]
	startGen := r.gens[organizationID]
	r.mu.Unlock()
	if ok && time.Since(e.builtAt) < cacheTTL && e.gen == startGen {
		return e.kb, nil
	}

	kb, err := r.Inner.Load(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	// Only cache this build if no Invalidate landed for this org while the
	// DB read above was in flight — otherwise a slow read started before an
	// edit would overwrite the fresh invalidation with stale data.
	if r.gens[organizationID] == startGen {
		r.entries[organizationID] = cacheEntry{kb: kb, builtAt: time.Now(), gen: startGen}
	}
	r.mu.Unlock()
	return kb, nil
}

// Invalidate drops orgID's cached entry, if any, so the next Load re-reads
// Postgres instead of serving a stale build. Safe to call for an org with no
// cached entry (a plain no-op). Also bumps the org's generation so any Load
// already in flight (its DB read started before this call) discards its
// result instead of caching it.
func (r *CachedKBRepo) Invalidate(orgID uuid.UUID) {
	key := orgID.String()
	r.mu.Lock()
	delete(r.entries, key)
	r.gens[key]++
	r.mu.Unlock()
}
