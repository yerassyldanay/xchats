package extractor

import "testing"

func TestImagesFromMarkdown(t *testing.T) {
	md := "Текст перед.\n" +
		"![](https://cdn.example.com/no-alt.png)\n" +
		"![С подписью](https://cdn.example.com/captioned.jpg \"необязательный заголовок\")\n" +
		"[Обычная ссылка, не картинка](https://example.com/page)\n" +
		"![Ещё одна](/relative/path.png)\n"

	got := imagesFromMarkdown(md)
	if len(got) != 3 {
		t.Fatalf("imagesFromMarkdown() = %+v, want 3 images (the plain link must not match)", got)
	}
	if got[0].URL != "https://cdn.example.com/no-alt.png" || got[0].Alt != "" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].URL != "https://cdn.example.com/captioned.jpg" || got[1].Alt != "С подписью" {
		t.Errorf("got[1] = %+v, want the title clause stripped from the URL capture", got[1])
	}
	if got[2].URL != "/relative/path.png" {
		t.Errorf("got[2].URL = %q, want the raw relative path (resolution is images.go's job, not markdown.go's)", got[2].URL)
	}
}

func TestImagesFromMarkdown_NoImages(t *testing.T) {
	if got := imagesFromMarkdown("Просто текст без картинок."); got != nil {
		t.Errorf("imagesFromMarkdown(no images) = %+v, want nil", got)
	}
}
