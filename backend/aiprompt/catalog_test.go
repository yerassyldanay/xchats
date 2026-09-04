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
	// kettle: available, no media columns set at all (media-less).
	kb.Products = append(kb.Products, Product{
		Ref: "kettle", Name: "Чайник", Price: "5 900 ₸", AvailabilityStatus: "in_stock", SalesStatus: "active",
	})
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

	foundKettleAbsent := false
	for _, a := range cat.Absent {
		if a.Table == "products" && a.Ref == "kettle" {
			foundKettleAbsent = true
		}
	}
	if !foundKettleAbsent {
		t.Error("kettle has no media columns populated and must appear in Absent")
	}
	if got := cat.MediaByToken("products.kettle.featured_image"); got != nil {
		t.Error("kettle has no featured_image; must not appear in media catalog")
	}

	// cookware-set is unavailable (baseKB): complete suppression means it gets
	// no media token AND no Absent entry either — it never gets a rendered
	// block at all (renderProductsUnavailable is name-only), so there is
	// nothing for Absent to say about it.
	for _, a := range cat.Absent {
		if a.Table == "products" && a.Ref == "cookware-set" {
			t.Error("unavailable product cookware-set must not appear in Absent — it is fully suppressed, not media-less")
		}
	}
	if got := cat.MediaByToken("products.cookware-set.featured_image"); got != nil {
		t.Error("unavailable product cookware-set must not appear in the media catalog even when a column is populated")
	}

	// Partial population: only featured_image set, gallery empty -> catalogued
	// for featured_image only, and NOT listed as media-less.
	kb2 := baseKB()
	kb2.Products = append(kb2.Products, Product{
		Ref: "kettle", Name: "Чайник", Price: "5 900 ₸", AvailabilityStatus: "in_stock", SalesStatus: "active",
		FeaturedImage: "m-cm-featured", // reuse a valid material id
	})
	cat2, err := BuildCatalog(kb2)
	if err != nil {
		t.Fatalf("BuildCatalog (partial): %v", err)
	}
	if got := cat2.MediaByToken("products.kettle.featured_image"); got == nil {
		t.Error("expected products.kettle.featured_image after partial population")
	}
	if got := cat2.MediaByToken("products.kettle.gallery_images"); got != nil {
		t.Error("kettle.gallery_images still empty; must not be catalogued")
	}
	for _, a := range cat2.Absent {
		if a.Table == "products" && a.Ref == "kettle" {
			t.Error("kettle now has one populated media column; must not be in Absent")
		}
	}
}

// TestBuildCatalog_FeaturedImageOverrideWithFallback covers
// resolvedFeaturedImage's rule: an explicit featured_image always wins; a
// blank featured_image resolves to the type's primary plural image column's
// first entry when that is non-empty; with neither, there is no token at
// all. Exercised across all three types that carry featured_image
// (products, topics, tariffs) — the rule is not product-specific.
func TestBuildCatalog_FeaturedImageOverrideWithFallback(t *testing.T) {
	t.Run("product: explicit value wins over gallery_images[0]", func(t *testing.T) {
		kb := baseKB() // coffee-machine: FeaturedImage=m-cm-featured, GalleryImages=[m-cm-gallery-1, m-cm-gallery-2]
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}
		entry := cat.MediaByToken("products.coffee-machine.featured_image")
		if entry == nil || entry.Count != 1 {
			t.Fatalf("expected featured_image present with count 1, got %+v", entry)
		}
		resolved, err := ResolveSend([]string{"products.coffee-machine.featured_image"}, kb, cat)
		if err != nil || len(resolved) != 1 || resolved[0].Material.ID != "m-cm-featured" {
			t.Fatalf("expected the EXPLICIT material, got %+v (err=%v)", resolved, err)
		}
	})

	t.Run("product: blank featured_image falls back to gallery_images[0]", func(t *testing.T) {
		kb := baseKB()
		kb.Products[0].FeaturedImage = ""
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}
		entry := cat.MediaByToken("products.coffee-machine.featured_image")
		if entry == nil || entry.Count != 1 {
			t.Fatalf("expected featured_image present (via fallback) with count 1, got %+v", entry)
		}
		resolved, err := ResolveSend([]string{"products.coffee-machine.featured_image"}, kb, cat)
		if err != nil || len(resolved) != 1 || resolved[0].Material.ID != "m-cm-gallery-1" {
			t.Fatalf("expected the FALLBACK material (gallery_images[0]), got %+v (err=%v)", resolved, err)
		}
	})

	t.Run("product: blank featured_image AND empty gallery_images means no token", func(t *testing.T) {
		kb := baseKB()
		kb.Products[0].FeaturedImage = ""
		kb.Products[0].GalleryImages = nil
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}
		if got := cat.MediaByToken("products.coffee-machine.featured_image"); got != nil {
			t.Fatalf("expected no featured_image token with neither an explicit value nor a fallback, got %+v", got)
		}
	})

	t.Run("topic: blank featured_image falls back to illustration_images[0]", func(t *testing.T) {
		kb := baseKB()
		kb.Topics[0].IllustrationImages = []string{"m-cm-gallery-1"}
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}
		if got := cat.MediaByToken("topics.delivery.featured_image"); got == nil {
			t.Fatalf("expected topics.delivery.featured_image via illustration_images fallback")
		}
	})

	t.Run("tariff: blank featured_image falls back to pricing_images[0]", func(t *testing.T) {
		kb := baseKB()
		kb.Tariffs[0].PricingImages = []string{"m-cm-gallery-1"}
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}
		if got := cat.MediaByToken("tariffs.basic.featured_image"); got == nil {
			t.Fatalf("expected tariffs.basic.featured_image via pricing_images fallback")
		}
	})
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

// TestBuildCatalog_NoInStockFactToken is the invariant established by removing
// in_stock as a fact column entirely (2026-07-26, registry.go): a token can't
// carry a value whole sentences grammatically depend on (the negation breaks
// "Товар {{...}}", and follow-on sentences like "Могу оформить заказ" are only
// valid in one state). BuildCatalog must never produce an in_stock fact for any
// product, in-stock or out-of-stock — availability is now expressed only via
// which of PRODUCTS IN STOCK / PRODUCTS OUT OF STOCK a product appears in
// (prompt.go's renderProductsInStock/renderProductsOutOfStock), never a
// citable placeholder. The same now holds for availability_status
// (0017_kb_virtual_facts): it is rendered as plain visible text in a v6
// product block (availabilityStatusRU, prompt.go), never a hidden token.
func TestBuildCatalog_NoInStockFactToken(t *testing.T) {
	kb := baseKB() // coffee-machine AvailabilityStatus="in_stock", cookware-set "unavailable"
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if f := cat.FactByToken("{{product.coffee-machine.in_stock}}"); f != nil {
		t.Errorf("expected no in_stock fact for an in-stock product, got %+v", f)
	}
	if f := cat.FactByToken("{{product.cookware-set.in_stock}}"); f != nil {
		t.Errorf("expected no in_stock fact for an out-of-stock product, got %+v", f)
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
