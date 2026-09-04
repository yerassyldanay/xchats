package kbstore

import (
	"fmt"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// facts_validate.go runs aiprompt.ValidateFacts against a product/tariff/
// tariff_info's CURRENT merged shape (the patch already applied) at every
// write path that can change AdditionalFacts or any of the prose fields the
// leak check compares against — UpsertProduct/UpsertTariff (draft.go),
// MCPUpsertProduct/MCPUpsertTariff/MCPUpsertTariffInfo (mcp_write.go), and
// PutLiveProduct/PutLiveTariff/PatchLiveTariffInfo (live.go) — so an invalid
// fact list is rejected at the moment it is staged, never merely caught
// later at prompt-render time (aiprompt.BuildCatalog re-validates too, but
// only as defense in depth: see facts.go's doc comment).
//
// productConcreteRefs/tariffConcreteRefs mirror aiprompt's own
// productConcreteColumns/tariffConcreteColumns (unexported there); kept as
// a second, explicit literal here rather than exporting aiprompt's — the
// collision list is part of the registry's closed vocabulary and small
// enough that duplicating the two short slices is clearer than adding an
// exported accessor solely for this one call site.
var (
	productConcreteRefs = []string{"price"}
	tariffConcreteRefs  = []string{"price", "fee"}
)

func validateProductFacts(p DraftProduct) error {
	prose := map[string]string{
		"description":        p.Description,
		"advantages":         p.Advantages,
		"disadvantages":      p.Disadvantages,
		"best_for":           p.BestFor,
		"not_for":            p.NotFor,
		"availability_note":  p.AvailabilityNote,
		"installation_terms": p.InstallationTerms,
		"warranty_terms":     p.WarrantyTerms,
	}
	if err := aiprompt.ValidateFacts(p.AdditionalFacts, productConcreteRefs, prose); err != nil {
		return fmt.Errorf("kbstore: product %s: %w", p.Ref, err)
	}
	return nil
}

func validateTariffFacts(t DraftTariff) error {
	prose := map[string]string{
		"summary":       t.Summary,
		"limit_text":    t.LimitText,
		"advantages":    t.Advantages,
		"disadvantages": t.Disadvantages,
		"best_for":      t.BestFor,
		"not_for":       t.NotFor,
	}
	if err := aiprompt.ValidateFacts(t.AdditionalFacts, tariffConcreteRefs, prose); err != nil {
		return fmt.Errorf("kbstore: tariff %s: %w", t.Ref, err)
	}
	return nil
}

func validateTariffInfoFacts(ti DraftTariffInfo) error {
	if err := aiprompt.ValidateFacts(ti.AdditionalFacts, nil, nil); err != nil {
		return fmt.Errorf("kbstore: tariff_info: %w", err)
	}
	return nil
}
