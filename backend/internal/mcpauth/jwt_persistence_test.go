package mcpauth_test

// jwt_persistence_test.go covers Task 17's release gate: a signing key
// derived from a PERSISTENT seed (MCP_JWT_SIGNING_KEY) must let tokens
// survive both a process restart and a multi-replica deployment — the two
// scenarios NewEphemeralSigningKey cannot support, since each call mints an
// unrelated random keypair. Both scenarios are structurally the same test:
// two independently constructed SigningKey values from the same seed must
// accept each other's tokens.

import (
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
)

const testPersistentSeed = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestSigningKey_PersistentSeedSurvivesRestartAndMultipleReplicas(t *testing.T) {
	// "Restart": the same process reloads the env var and rebuilds its key.
	keyBeforeRestart, err := mcpauth.NewSigningKeyFromSeed(testPersistentSeed)
	if err != nil {
		t.Fatalf("key before restart: %v", err)
	}
	keyAfterRestart, err := mcpauth.NewSigningKeyFromSeed(testPersistentSeed)
	if err != nil {
		t.Fatalf("key after restart: %v", err)
	}

	// "Multi-replica": a second process, same configured seed, at the same time.
	keyOnReplicaTwo, err := mcpauth.NewSigningKeyFromSeed(testPersistentSeed)
	if err != nil {
		t.Fatalf("key on replica two: %v", err)
	}

	claims := mcpauth.Claims{
		Issuer: "https://xchats.test", Audience: "https://xchats.test/mcp",
		Subject: "11111111-1111-1111-1111-111111111111",
		IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix(),
		JTI: "persistence-check-jti",
	}
	token, err := keyBeforeRestart.Sign(claims)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := keyAfterRestart.Verify(token, claims.Issuer, claims.Audience); err != nil {
		t.Fatalf("a token signed before a restart must still verify after one (same persistent seed): %v", err)
	}
	if _, err := keyOnReplicaTwo.Verify(token, claims.Issuer, claims.Audience); err != nil {
		t.Fatalf("a token minted by one replica must verify on another (same persistent seed): %v", err)
	}
}

// TestSigningKey_EphemeralKeysAreProcessLocal is the contrasting negative
// case: NewEphemeralSigningKey's whole point is that no two calls ever agree
// — this is what makes it unsafe for production (plan Task 17), and this
// test would fail (once in a vanishingly unlikely while, never in practice —
// Ed25519 keys are 256 bits) if that ever stopped being true.
func TestSigningKey_EphemeralKeysAreProcessLocal(t *testing.T) {
	keyA := mcpauth.NewEphemeralSigningKey()
	keyB := mcpauth.NewEphemeralSigningKey()

	claims := mcpauth.Claims{
		Issuer: "https://xchats.test", Audience: "https://xchats.test/mcp",
		Subject: "11111111-1111-1111-1111-111111111111",
		IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix(),
		JTI: "ephemeral-check-jti",
	}
	token, err := keyA.Sign(claims)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := keyB.Verify(token, claims.Issuer, claims.Audience); err == nil {
		t.Fatalf("two independently generated ephemeral keys must NOT cross-verify a token")
	}
}
