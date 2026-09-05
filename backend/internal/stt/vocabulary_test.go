package stt

import (
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

func TestBuildPrompt_CombinesCatalogAndCustomVocabulary(t *testing.T) {
	kb := &aiprompt.KB{
		Products: []aiprompt.Product{
			{Name: "iPhone 15 Pro", Category: "Смартфоны", Brand: "Apple", SalesStatus: "active"},
			{Name: "Дискontinued", Category: "Старое", SalesStatus: "inactive"},
		},
		Tariffs: []aiprompt.Tariff{
			{Name: "Безлимит XL", SalesStatus: "active"},
			{Name: "Старый тариф", SalesStatus: "inactive"},
		},
	}
	prompt := BuildPrompt(kb, "Kaspi, kolesa.kz")

	for _, want := range []string{"Kaspi", "kolesa.kz", "iPhone 15 Pro", "Безлимит XL", "Смартфоны", "Apple"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt %q missing %q", prompt, want)
		}
	}
	for _, notWant := range []string{"Дискontinued", "Старое", "Старый тариф"} {
		if strings.Contains(prompt, notWant) {
			t.Errorf("prompt %q must not include inactive listing %q", prompt, notWant)
		}
	}
}

func TestBuildPrompt_CustomVocabularyComesFirst(t *testing.T) {
	kb := &aiprompt.KB{Products: []aiprompt.Product{{Name: "Product One", SalesStatus: "active"}}}
	prompt := BuildPrompt(kb, "MyBrand")
	if i, j := strings.Index(prompt, "MyBrand"), strings.Index(prompt, "Product One"); i < 0 || j < 0 || i > j {
		t.Errorf("prompt = %q, want custom vocabulary before catalog terms", prompt)
	}
}

func TestBuildPrompt_DeduplicatesCaseInsensitively(t *testing.T) {
	kb := &aiprompt.KB{Products: []aiprompt.Product{{Name: "Kaspi", SalesStatus: "active"}}}
	prompt := BuildPrompt(kb, "kaspi, KASPI")
	if n := strings.Count(strings.ToLower(prompt), "kaspi"); n != 1 {
		t.Errorf("prompt = %q, want exactly one case-insensitive occurrence of kaspi, got %d", prompt, n)
	}
}

func TestBuildPrompt_NilKBFallsBackToCustomVocabulary(t *testing.T) {
	prompt := BuildPrompt(nil, "one, two")
	if prompt != "one, two" {
		t.Errorf("prompt = %q, want %q", prompt, "one, two")
	}
}

func TestBuildPrompt_EmptyInputsYieldEmptyString(t *testing.T) {
	if prompt := BuildPrompt(nil, ""); prompt != "" {
		t.Errorf("prompt = %q, want empty", prompt)
	}
	if prompt := BuildPrompt(&aiprompt.KB{}, ""); prompt != "" {
		t.Errorf("prompt = %q, want empty", prompt)
	}
}

func TestBuildPrompt_CapsLength(t *testing.T) {
	var products []aiprompt.Product
	for i := 0; i < 500; i++ {
		products = append(products, aiprompt.Product{Name: strings.Repeat("x", 20) + string(rune('a'+i%26)), SalesStatus: "active"})
	}
	prompt := BuildPrompt(&aiprompt.KB{Products: products}, "")
	if got := len([]rune(prompt)); got > maxPromptRunes {
		t.Errorf("prompt length = %d runes, want <= %d", got, maxPromptRunes)
	}
}
