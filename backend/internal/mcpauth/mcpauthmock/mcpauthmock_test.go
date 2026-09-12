package mcpauthmock

import (
	"context"
	"testing"
)

func TestFetchCIMDReturnsAValidClient(t *testing.T) {
	got, err := FetchCIMD(context.Background(), "https://client.example/metadata.json")
	if err != nil {
		t.Fatalf("FetchCIMD: %v", err)
	}
	if got.ClientID == "" || got.ClientName == "" || len(got.RedirectURIs) == 0 || got.Source != "cimd" {
		t.Fatalf("got %+v, want every field populated and Source=cimd", got)
	}
}

func TestFetchCIMDRespectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FetchCIMD(ctx, "https://client.example/metadata.json"); err == nil {
		t.Fatal("expected an error for an already-canceled context")
	}
}
