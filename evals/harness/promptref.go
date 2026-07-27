package main

import "fmt"

// preprocessorImageJPEG85 names the current (and, for now, only) input pipeline:
// decode -> downscale to maxImageSide -> re-encode as JPEG q85 (imgscale.go). Recorded
// on every extractRunResult so a future second image pipeline, or a non-image kind, is
// distinguishable in results instead of silently assumed to be this one.
const preprocessorImageJPEG85 = "image_1024_jpeg85"

// preprocess dispatches on an ExtractCase's Kind (empty defaults to "image") to the
// loader for that input type. Only "image" exists today; this function is the seam a
// future "pdf" | "audio" | "url" kind plugs into. An unrecognized kind is a hard error
// — fail-closed, never a silent fall-through to the image path. It stays in package
// main (unlike PromptRef/LoadPrompt, which moved to internal/provenance) because it
// dispatches into loadAndScaleImage — extraction-family logic, not generic provenance.
func preprocess(kind, path string) (data []byte, mimetype, preprocessorName string, err error) {
	switch kind {
	case "", "image":
		data, mimetype, err = loadAndScaleImage(path)
		return data, mimetype, preprocessorImageJPEG85, err
	default:
		return nil, "", "", fmt.Errorf("preprocess: unsupported case kind %q (only \"image\" is implemented)", kind)
	}
}
