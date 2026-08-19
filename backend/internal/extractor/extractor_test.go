package extractor_test

import (
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/extractor"
)

func TestRegistry_GetAndNames(t *testing.T) {
	native := extractor.NewNative(testClient(), 1<<20)
	fc := extractor.NewFirecrawl("key", "", testClient())
	reg := extractor.NewRegistry(native, fc)

	if names := reg.Names(); len(names) != 2 || names[0] != "native" || names[1] != "firecrawl" {
		t.Fatalf("Names() = %v, want [native firecrawl] in registration order", names)
	}
	if _, ok := reg.Get("native"); !ok {
		t.Error("expected native to resolve")
	}
	if _, ok := reg.Get("llamaparse"); ok {
		t.Error("expected llamaparse to be absent — it was never registered")
	}
}
