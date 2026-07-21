package kbfixture

import (
	"os"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// committedFixturePath is the generator's real output target, relative to
// this package directory.
const committedFixturePath = "../../../scenarios/shop-kb-v1/data-ru.yaml"

// TestGenerate_StructuralShape pins the counts and pinned-row content the
// rest of the shop-kb-v1 family (frame, question bank, size-limited
// scenarios) is authored against, independent of the exact YAML bytes.
func TestGenerate_StructuralShape(t *testing.T) {
	fx := Generate()

	if len(fx.AITopics) != 9 {
		t.Errorf("want 9 topics, got %d", len(fx.AITopics))
	}
	if len(fx.AIProducts) != 100 {
		t.Fatalf("want 100 products, got %d", len(fx.AIProducts))
	}

	cm := fx.AIProducts[0]
	if cm.Ref != "coffee-machine" || cm.InStock == nil || !bool(*cm.InStock) {
		t.Errorf("want product[0] = in-stock coffee-machine, got ref=%q in_stock=%v", cm.Ref, cm.InStock)
	}
	if cm.FeaturedImage == "" || len(cm.GalleryImages) == 0 || len(cm.DemoVideos) == 0 || len(cm.CertificateDocuments) == 0 {
		t.Errorf("want product[0] to carry featured+gallery+video+certificate media, got %+v", cm)
	}

	cw := fx.AIProducts[1]
	if cw.Ref != "cookware-set" || cw.InStock == nil || bool(*cw.InStock) {
		t.Errorf("want product[1] = out-of-stock cookware-set, got ref=%q in_stock=%v", cw.Ref, cw.InStock)
	}
	if len(cw.GalleryImages) == 0 {
		t.Errorf("want product[1] to carry gallery media, got %+v", cw)
	}
	if cw.FeaturedImage != "" || len(cw.DemoVideos) != 0 || len(cw.CertificateDocuments) != 0 {
		t.Errorf("want product[1] to carry ONLY gallery media, got %+v", cw)
	}

	if fx.AIPolicies == nil || fx.AIPolicies.ReturnPeriodInDays != "" {
		t.Errorf("want ai_policies.return_period_in_days deliberately empty, got %+v", fx.AIPolicies)
	}

	seenRef := map[string]bool{}
	inStockTrue, withMedia := 0, 0
	for i, p := range fx.AIProducts {
		if seenRef[p.Ref] {
			t.Errorf("duplicate product ref %q at index %d", p.Ref, i)
		}
		seenRef[p.Ref] = true
		if p.InStock != nil && bool(*p.InStock) {
			inStockTrue++
		}
		if p.FeaturedImage != "" || len(p.GalleryImages) > 0 || len(p.DemoVideos) > 0 || len(p.CertificateDocuments) > 0 {
			withMedia++
		}
	}
	// index%3==0 is out of stock among the 98 generated rows (index 2..99),
	// plus the pinned rows override the formula in both directions — see
	// mediaPlanFor's doc comment for the exact split this pins loosely
	// (a wide band, not the precise count, so the rotation formula stays
	// free to shift without turning every tweak into a test edit).
	if inStockTrue < 50 || inStockTrue > 80 {
		t.Errorf("want a majority but not all products in stock, got %d/100 in stock", inStockTrue)
	}
	if withMedia < 40 || withMedia > 60 {
		t.Errorf("want roughly half the products carrying media, got %d/100", withMedia)
	}
}

// TestGenerate_RoundTripsThroughStrictDecode proves the generated YAML is
// itself a valid, strictly-decodable shop-kb-v1 fixture (same Decode path a
// real scenario render uses) and that every media reference it contains
// resolves fail-closed through aiprompt.BuildCatalog — i.e. the generator
// can never accidentally emit a dangling or invalid reference.
func TestGenerate_RoundTripsThroughStrictDecode(t *testing.T) {
	out, err := GenerateYAML()
	if err != nil {
		t.Fatalf("GenerateYAML: %v", err)
	}

	kb, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode(GenerateYAML()): %v", err)
	}
	if len(kb.Products) != 100 {
		t.Errorf("want 100 products after round-trip, got %d", len(kb.Products))
	}

	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog(round-tripped KB): %v", err)
	}
	if len(cat.Media) == 0 {
		t.Error("want at least one resolved media entry, got zero")
	}
	if len(cat.Absent) == 0 {
		t.Error("want at least one media-less record (absent entry), got zero")
	}

	// Every referenced material must resolve exactly once; the two
	// deliberately unreferenced materials must NOT surface as a token.
	referenced := map[string]bool{}
	for _, p := range kb.Products {
		for _, id := range allProductMaterialIDs(p) {
			referenced[id] = true
		}
	}
	for _, tp := range kb.Topics {
		for _, id := range tp.IllustrationImages {
			referenced[id] = true
		}
	}
	if len(referenced) == 0 {
		t.Fatal("expected at least one referenced material id")
	}
	unreferencedCount := 0
	for _, m := range kb.Materials {
		if !referenced[m.ID] {
			unreferencedCount++
		}
	}
	if unreferencedCount != 2 {
		t.Errorf("want exactly 2 deliberately unreferenced materials, got %d", unreferencedCount)
	}
}

func allProductMaterialIDs(p aiprompt.Product) []string {
	var ids []string
	if p.FeaturedImage != "" {
		ids = append(ids, p.FeaturedImage)
	}
	ids = append(ids, p.GalleryImages...)
	ids = append(ids, p.DemoVideos...)
	ids = append(ids, p.AudioDescriptionFiles...)
	ids = append(ids, p.CertificateDocuments...)
	ids = append(ids, p.ManualDocuments...)
	ids = append(ids, p.GuaranteeDocuments...)
	ids = append(ids, p.SpecificationDocuments...)
	return ids
}

// TestGenerate_MatchesCommittedFixture is the byte-for-byte drift check: the
// committed evals/scenarios/shop-kb-v1/data-ru.yaml must always be exactly
// what running the generator produces right now. A failure here means
// someone hand-edited the committed file, or changed the generator without
// regenerating it — either way, run `go run ./cmd/genkbfixture` from
// evals/harness and commit the result.
func TestGenerate_MatchesCommittedFixture(t *testing.T) {
	committed, err := os.ReadFile(committedFixturePath)
	if err != nil {
		t.Fatalf("read committed fixture %s: %v (run `go run ./cmd/genkbfixture` from evals/harness first)", committedFixturePath, err)
	}
	generated, err := GenerateYAML()
	if err != nil {
		t.Fatalf("GenerateYAML: %v", err)
	}
	if string(generated) != string(committed) {
		t.Errorf("committed %s is stale: does not match the generator's current output.\n"+
			"Run `cd evals/harness && go run ./cmd/genkbfixture` and commit the result.", committedFixturePath)
	}
}
