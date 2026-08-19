package extractor

import "regexp"

// markdownImageRE matches a markdown image reference: ![alt](url "title")
// with the title clause optional. Deliberately simple (no nested brackets,
// no reference-style [alt][ref] links) — Firecrawl/LlamaParse both emit
// plain inline image syntax, and this only needs to find image URLs to
// stage, not round-trip arbitrary markdown.
var markdownImageRE = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

// imagesFromMarkdown extracts every ![alt](url) reference from markdown
// text, in document order — used by firecrawl.go and llamaparse.go, whose
// providers both return markdown as their extraction format
// (plan/DECISIONS.md: "All extractors emit the same evidence shape", so a
// markdown-based provider's images travel through the same Result.Images
// field the native HTML provider's <img> tags do).
func imagesFromMarkdown(md string) []Image {
	matches := markdownImageRE.FindAllStringSubmatch(md, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]Image, 0, len(matches))
	for _, m := range matches {
		alt, src := m[1], m[2]
		if src == "" {
			continue
		}
		out = append(out, Image{URL: src, Alt: alt})
	}
	return out
}
