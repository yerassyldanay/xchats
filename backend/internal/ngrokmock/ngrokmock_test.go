package ngrokmock

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/ngrokapi"
)

func TestListReservedDomainsReturnsOnePlausibleDomain(t *testing.T) {
	domains, err := New().ListReservedDomains(context.Background(), "any-api-key")
	if err != nil {
		t.Fatalf("ListReservedDomains: %v", err)
	}
	if len(domains) != 1 || domains[0].Domain == "" {
		t.Fatalf("expected exactly one plausible reserved domain, got %+v", domains)
	}
}

// TestSelectDefaultDomainAcceptsIt proves the mock's shape survives the SAME
// validation applyNgrokPublicOrigin runs it through in production —
// ngrokapi.SelectDefaultDomain's own tie-breaking/whitespace handling.
func TestSelectDefaultDomainAcceptsIt(t *testing.T) {
	domains, err := New().ListReservedDomains(context.Background(), "any-api-key")
	if err != nil {
		t.Fatalf("ListReservedDomains: %v", err)
	}
	got, err := ngrokapi.SelectDefaultDomain(domains)
	if err != nil {
		t.Fatalf("SelectDefaultDomain: %v", err)
	}
	if got != Domain {
		t.Errorf("SelectDefaultDomain = %q, want %q", got, Domain)
	}
}

func TestListReservedDomainsRespectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().ListReservedDomains(ctx, "any-api-key"); err == nil {
		t.Fatal("expected an error for an already-canceled context")
	}
}
