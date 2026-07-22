package aiprompt

import "testing"

// TestBuildCatalog_MaterialFailClosed covers every fail-closed material rule:
// a non-empty media column referencing an invalid material must fail the
// entire BuildCatalog call, never silently drop the column or the reference.
func TestBuildCatalog_MaterialFailClosed(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*KB)
	}{
		{
			name: "nonexistent material ID",
			mutate: func(kb *KB) {
				kb.Products[0].FeaturedImage = "does-not-exist"
			},
		},
		{
			name: "material from another organization",
			mutate: func(kb *KB) {
				for i, m := range kb.Materials {
					if m.ID == "m-cm-featured" {
						kb.Materials[i].OrganizationID = otherOrg
					}
				}
			},
		},
		{
			name: "non-file source type",
			mutate: func(kb *KB) {
				setMaterial(kb, "m-cm-featured", func(m *Material) { m.SourceType = "url" })
			},
		},
		{
			name: "missing storage backend",
			mutate: func(kb *KB) {
				setMaterial(kb, "m-cm-featured", func(m *Material) { m.StorageBackend = "" })
			},
		},
		{
			name: "missing storage key",
			mutate: func(kb *KB) {
				setMaterial(kb, "m-cm-featured", func(m *Material) { m.StorageKey = "" })
			},
		},
		{
			name: "zero size",
			mutate: func(kb *KB) {
				setMaterial(kb, "m-cm-featured", func(m *Material) { m.SizeBytes = 0 })
			},
		},
		{
			name: "negative size",
			mutate: func(kb *KB) {
				setMaterial(kb, "m-cm-featured", func(m *Material) { m.SizeBytes = -1 })
			},
		},
		{
			name: "invisible customer_visibility",
			mutate: func(kb *KB) {
				setMaterial(kb, "m-cm-featured", func(m *Material) { m.CustomerVisibility = "invisible" })
			},
		},
		{
			name: "empty customer_visibility",
			mutate: func(kb *KB) {
				setMaterial(kb, "m-cm-featured", func(m *Material) { m.CustomerVisibility = "" })
			},
		},
		{
			name: "MIME type incompatible with column kind",
			mutate: func(kb *KB) {
				setMaterial(kb, "m-cm-featured", func(m *Material) { m.MimeType = "video/mp4" })
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kb := baseKB()
			tc.mutate(kb)
			cat, err := BuildCatalog(kb)
			if err == nil {
				t.Fatalf("BuildCatalog: expected error, got catalog with %d media entries", len(cat.Media))
			}
		})
	}
}

// setMaterial mutates the material with the given id in place.
func setMaterial(kb *KB, id string, fn func(*Material)) {
	for i := range kb.Materials {
		if kb.Materials[i].ID == id {
			fn(&kb.Materials[i])
			return
		}
	}
	panic("setMaterial: no material " + id)
}

// TestBuildCatalog_MaterialVisibilityAccepted confirms both "visible" and
// "auto" materials are customer-sendable on the live path; only "invisible"
// and empty are rejected (covered above).
func TestBuildCatalog_MaterialVisibilityAccepted(t *testing.T) {
	for _, vis := range []string{"visible", "auto"} {
		t.Run(vis, func(t *testing.T) {
			kb := baseKB()
			setMaterial(kb, "m-cm-featured", func(m *Material) { m.CustomerVisibility = vis })
			cat, err := BuildCatalog(kb)
			if err != nil {
				t.Fatalf("BuildCatalog: unexpected error for customer_visibility=%s: %v", vis, err)
			}
			if cat.MediaByToken("products.coffee-machine.featured_image") == nil {
				t.Fatalf("expected featured_image token in catalog for customer_visibility=%s", vis)
			}
		})
	}
}

// TestBuildCatalog_MediaAbsenceRule: a record is "without media" only when
// ALL of its media columns are empty; a partially populated record catalogs
// only its populated columns and is not media-less.
func TestBuildCatalog_MediaAbsenceRule(t *testing.T) {
	kb := baseKB()
	// coffee-machine: featured_image + gallery_images populated (fully populated).
	// cookware-set: no media columns set at all (fully media-less, from baseKB).
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	if got := cat.MediaByToken("products.coffee-machine.featured_image"); got == nil {
		t.Error("expected products.coffee-machine.featured_image in media catalog")
	}
	if got := cat.MediaByToken("products.coffee-machine.gallery_images"); got == nil {
		t.Error("expected products.coffee-machine.gallery_images in media catalog")
	} else if got.Count != 2 {
		t.Errorf("gallery_images count = %d, want 2", got.Count)
	}
	for _, a := range cat.Absent {
		if a.Table == "products" && a.Ref == "coffee-machine" {
			t.Error("coffee-machine has populated media columns and must not appear in Absent")
		}
	}

	foundCookwareAbsent := false
	for _, a := range cat.Absent {
		if a.Table == "products" && a.Ref == "cookware-set" {
			foundCookwareAbsent = true
		}
	}
	if !foundCookwareAbsent {
		t.Error("cookware-set has no media columns populated and must appear in Absent")
	}
	if got := cat.MediaByToken("products.cookware-set.featured_image"); got != nil {
		t.Error("cookware-set has no featured_image; must not appear in media catalog")
	}

	// Partial population: only featured_image set, gallery empty -> catalogued
	// for featured_image only, and NOT listed as media-less.
	kb2 := baseKB()
	kb2.Products[1].FeaturedImage = "m-cm-featured" // reuse a valid material id
	cat2, err := BuildCatalog(kb2)
	if err != nil {
		t.Fatalf("BuildCatalog (partial): %v", err)
	}
	if got := cat2.MediaByToken("products.cookware-set.featured_image"); got == nil {
		t.Error("expected products.cookware-set.featured_image after partial population")
	}
	if got := cat2.MediaByToken("products.cookware-set.gallery_images"); got != nil {
		t.Error("cookware-set.gallery_images still empty; must not be catalogued")
	}
	for _, a := range cat2.Absent {
		if a.Table == "products" && a.Ref == "cookware-set" {
			t.Error("cookware-set now has one populated media column; must not be in Absent")
		}
	}
}

// TestBuildCatalog_BlankExactFactsProduceNoPlaceholder: a missing or
// whitespace-only exact value must not generate a fact-catalog entry.
func TestBuildCatalog_BlankExactFactsProduceNoPlaceholder(t *testing.T) {
	kb := baseKB()
	kb.Products[0].Price = "   " // whitespace-only
	kb.Policies.ReturnPeriodInDays = ""
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if f := cat.FactByToken("{{product.coffee-machine.price}}"); f != nil {
		t.Errorf("expected no price fact for blank price, got %+v", f)
	}
	if f := cat.FactByToken("{{policy.main.return_period_in_days}}"); f != nil {
		t.Errorf("expected no return_period_in_days fact when empty, got %+v", f)
	}
	// Non-blank facts on the same rows must still render.
	if f := cat.FactByToken("{{policy.main.delivery_cost}}"); f == nil {
		t.Error("expected delivery_cost fact to still be present")
	}
}

// TestBuildCatalog_InStockSubstitution verifies both true and false produce a
// fact entry, and that the boolean itself (never a raw literal) drives the
// eventual Russian wording via SubstituteFacts (contract_test.go covers the
// substitution path itself; here we check the catalog-level representation).
func TestBuildCatalog_InStockSubstitution(t *testing.T) {
	kb := baseKB() // coffee-machine InStock=true, cookware-set InStock=false
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	inStock := cat.FactByToken("{{product.coffee-machine.in_stock}}")
	if inStock == nil {
		t.Fatal("expected in_stock fact for coffee-machine")
	}
	if inStock.Kind != KindStockFlag {
		t.Errorf("in_stock Kind = %v, want KindStockFlag", inStock.Kind)
	}
	outOfStock := cat.FactByToken("{{product.cookware-set.in_stock}}")
	if outOfStock == nil {
		t.Fatal("expected in_stock fact for cookware-set")
	}
	if inStock.ReasoningState != "in_stock" {
		t.Errorf("in-stock product ReasoningState = %q, want in_stock", inStock.ReasoningState)
	}
	if outOfStock.ReasoningState != "out_of_stock" {
		t.Errorf("out-of-stock product ReasoningState = %q, want out_of_stock", outOfStock.ReasoningState)
	}
}

// TestBuildCatalog_InactiveSalesStatusExcluded confirms inactive rows produce
// neither facts nor media/absence entries.
func TestBuildCatalog_InactiveSalesStatusExcluded(t *testing.T) {
	statuses := []struct {
		name   string
		status string
	}{
		{name: "empty", status: ""},
		{name: "inactive", status: "inactive"},
		{name: "pending", status: "pending"},
		{name: "wrong case", status: "ACTIVE"},
	}
	for _, tt := range statuses {
		t.Run(tt.name, func(t *testing.T) {
			kb := baseKB()
			kb.Products[0].SalesStatus = tt.status
			cat, err := BuildCatalog(kb)
			if err != nil {
				t.Fatalf("BuildCatalog: %v", err)
			}
			if f := cat.FactByToken("{{product.coffee-machine.price}}"); f != nil {
				t.Error("non-active product must not produce fact entries")
			}
			if m := cat.MediaByToken("products.coffee-machine.featured_image"); m != nil {
				t.Error("non-active product must not produce media entries")
			}
			for _, a := range cat.Absent {
				if a.Ref == "coffee-machine" {
					t.Error("non-active product must not appear in Absent")
				}
			}
		})
	}

	t.Run("active", func(t *testing.T) {
		cat, err := BuildCatalog(baseKB())
		if err != nil {
			t.Fatal(err)
		}
		if cat.FactByToken("{{product.coffee-machine.price}}") == nil {
			t.Error("active product must produce fact entries")
		}
		if cat.MediaByToken("products.coffee-machine.featured_image") == nil {
			t.Error("active product must produce media entries")
		}
	})
}
