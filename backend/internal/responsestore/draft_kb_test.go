package responsestore_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
)

func TestBuildKBFromDraftView_MapsEveryEntityKind(t *testing.T) {
	featuredImage := uuid.New()
	gallery := uuid.New()

	dv := &kbstore.DraftView{
		Config: kbstore.DraftConfig{
			Persona: "Дружелюбный ассистент.", Mission: "Помочь клиенту.",
			Guardrails: "Не обещать скидки.", LanguagePolicy: "ru", ReplyMaxWords: 80,
		},
		Topics: []kbstore.TopicRow{
			{Slug: "faq", Title: "Частые вопросы", BodyMD: "Текст.", FeaturedImage: &featuredImage},
		},
		Products: []kbstore.ProductRow{
			{Ref: "widget", Name: "Виджет", Price: "1000", InStock: true, SalesStatus: "active", GalleryImages: []uuid.UUID{gallery}},
		},
		Tariffs: []kbstore.TariffRow{
			{Ref: "pro", Name: "Про", Price: "5000", SalesStatus: "active"},
		},
		Zones: []kbstore.ZoneRow{
			{Ref: "almaty", Name: "Алматы", ZoneLevel: "city", DeliveryAvailable: true, DeliveryCost: "1000", DeliveryInDays: "1"},
		},
		Contacts: []kbstore.ContactRow{
			{Phone: "+7 700 000 00 00", Email: "shop@example.com"},
		},
		Policies: []kbstore.PolicyRow{
			{DeliveryCost: "1000", DeliveryInDays: "1-2", Warranty: "12 months"},
		},
		Materials: []kbstore.Material{
			{ID: featuredImage, SourceType: "file", Filename: "photo.png", MimeType: "image/png", ProcessingStatus: "parsed", CustomerVisibility: "visible"},
		},
	}

	kb := responsestore.BuildKBFromDraftView("org-1", dv)

	if kb.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want org-1", kb.OrganizationID)
	}
	if kb.Assistant == nil || kb.Assistant.Persona != "Дружелюбный ассистент." || kb.Assistant.ReplyMaxWords != 80 {
		t.Errorf("Assistant not mapped: %+v", kb.Assistant)
	}
	if len(kb.Topics) != 1 || kb.Topics[0].Slug != "faq" || kb.Topics[0].FeaturedImage != featuredImage.String() {
		t.Errorf("Topics not mapped: %+v", kb.Topics)
	}
	if len(kb.Products) != 1 || kb.Products[0].Ref != "widget" || len(kb.Products[0].GalleryImages) != 1 || kb.Products[0].GalleryImages[0] != gallery.String() {
		t.Errorf("Products not mapped: %+v", kb.Products)
	}
	if len(kb.Tariffs) != 1 || kb.Tariffs[0].Ref != "pro" {
		t.Errorf("Tariffs not mapped: %+v", kb.Tariffs)
	}
	if len(kb.DeliveryZones) != 1 || kb.DeliveryZones[0].Ref != "almaty" {
		t.Errorf("DeliveryZones not mapped: %+v", kb.DeliveryZones)
	}
	if kb.Contacts == nil || kb.Contacts.Phone != "+7 700 000 00 00" {
		t.Errorf("Contacts not mapped: %+v", kb.Contacts)
	}
	if kb.Policies == nil || kb.Policies.Warranty != "12 months" {
		t.Errorf("Policies not mapped: %+v", kb.Policies)
	}
	if len(kb.Materials) != 1 || kb.Materials[0].ID != featuredImage.String() || kb.Materials[0].Filename != "photo.png" {
		t.Errorf("Materials not mapped: %+v", kb.Materials)
	}
	// Documented, deliberate gap — see BuildKBFromDraftView's own doc comment.
	if kb.Materials[0].StorageBackend != "" || kb.Materials[0].StorageKey != "" {
		t.Errorf("expected blank storage fields (kbstore.Material never exposes them), got %+v", kb.Materials[0])
	}
}

func TestBuildKBFromDraftView_NoContactsOrPolicies_LeavesThemNil(t *testing.T) {
	kb := responsestore.BuildKBFromDraftView("org-1", &kbstore.DraftView{})
	if kb.Contacts != nil {
		t.Errorf("Contacts = %+v, want nil when the draft view carries none", kb.Contacts)
	}
	if kb.Policies != nil {
		t.Errorf("Policies = %+v, want nil when the draft view carries none", kb.Policies)
	}
	if kb.Assistant == nil {
		t.Error("Assistant must never be nil — a zero-valued DraftConfig still maps to a (blank) Assistant")
	}
}
