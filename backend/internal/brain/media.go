package brain

import (
	"embed"
	"fmt"
	"path"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
)

//go:embed kb-media/*
var kbMediaFS embed.FS

// kbAsset binds a catalog ref (== blob id, == SeedSnapshot Asset.Ref) to its
// embedded file + media metadata.
type kbAsset struct {
	ref, file, kind, mime string
}

var kbAssets = []kbAsset{
	{RefPricingCard, "pricing_card.png", "image", "image/png"},
	{RefCatalogPDF, "catalog_pdf.pdf", "document", "application/pdf"},
	{RefIntroAudio, "intro_audio.wav", "audio", "audio/wav"},
}

// LoadMedia writes the embedded KB media into the blob store (idempotent,
// deterministic ids) so that approving a draft can send the files by ref — the
// same pattern as the Stub's NewStub. Call once at boot when the real brain is active.
func LoadMedia(store blob.Store) error {
	for _, a := range kbAssets {
		data, err := kbMediaFS.ReadFile(path.Join("kb-media", a.file))
		if err != nil {
			return fmt.Errorf("read kb media %s: %w", a.file, err)
		}
		if _, err := store.Put(a.ref, data, blob.Meta{
			MediaType: a.kind, Mimetype: a.mime, FileName: a.file, FileSize: int64(len(data)),
		}); err != nil {
			return fmt.Errorf("put kb media %s: %w", a.ref, err)
		}
	}
	return nil
}
