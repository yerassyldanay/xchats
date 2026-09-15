package credentialsmock_test

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/credentials/credentialsmock"
)

// TestClient_EveryProviderValidatesSuccessfully drives the mock through the
// REAL credentials.Provider.Validate hooks (the exact code a Settings
// "Test connection" click runs) for every registered provider — proving
// the mock's 200-OK-for-everything answer is what classify() needs to call
// each one healthy, with no real network reachable.
func TestClient_EveryProviderValidatesSuccessfully(t *testing.T) {
	credentials.SetValidateHTTPClient(credentialsmock.Client())
	t.Cleanup(func() { credentials.SetValidateHTTPClient(nil) })

	for _, p := range credentials.Providers() {
		p := p
		t.Run(p.ID, func(t *testing.T) {
			values := map[credentials.Key]string{}
			for _, f := range p.Fields {
				values[f.Key] = "mock-value"
			}
			if err := p.Validate(context.Background(), values); err != nil {
				t.Errorf("Validate(%s) = %v, want nil (mock transport always answers 200)", p.ID, err)
			}
		})
	}
}
