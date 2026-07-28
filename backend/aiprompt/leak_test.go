package aiprompt

import "testing"

func TestValidateNoStorageLocatorLeak_Clean(t *testing.T) {
	if err := ValidateNoStorageLocatorLeak("Цена — {{product.x.price}}, спасибо."); err != nil {
		t.Fatalf("unexpected error on clean prompt: %v", err)
	}
}

func TestValidateNoStorageLocatorLeak_FakeScheme(t *testing.T) {
	if err := ValidateNoStorageLocatorLeak("текст fake://bucket/abc текст"); err == nil {
		t.Fatal("want error for fake:// locator, got nil")
	}
}

func TestValidateNoStorageLocatorLeak_NoHTTPCarveOut(t *testing.T) {
	// Locked decision: no http/https carve-out — fact values are token-hidden, so a
	// literal URL of any scheme has no legitimate reason to appear in a prompt.
	if err := ValidateNoStorageLocatorLeak("подробнее: https://example.com/path"); err == nil {
		t.Fatal("want error for https:// URL — no scheme carve-out is allowed")
	}
}

func TestValidateNoStorageLocatorLeak_S3Scheme(t *testing.T) {
	if err := ValidateNoStorageLocatorLeak("s3://my-bucket/key.png"); err == nil {
		t.Fatal("want error for s3:// locator, got nil")
	}
}
