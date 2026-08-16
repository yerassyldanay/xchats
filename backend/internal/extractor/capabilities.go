package extractor

// Family is a coarse source-kind/MIME grouping Capabilities probes each
// provider with — precise enough to drive an operator-facing capability
// matrix (plan/playground.md's per-input-type table, which
// capabilities_test.go asserts this matches) without enumerating the full
// MIME space Supports() itself accepts.
type Family string

const (
	FamilyURL   Family = "url"
	FamilyText  Family = "text"
	FamilyDOCX  Family = "docx"
	FamilyPDF   Family = "pdf"
	FamilyImage Family = "image"
)

// Families is every Family, in a fixed display order — the order
// ProviderCapabilities.Families lists a provider's supported ones in.
var Families = []Family{FamilyURL, FamilyText, FamilyDOCX, FamilyPDF, FamilyImage}

// probeSource is the one representative Source Capabilities feeds to
// Supports() for f — a real value from the family, not an edge case, so
// probing stays a faithful stand-in for "can this provider read this kind
// of thing." Only Kind/MimeType/URL matter: Supports() never inspects Data.
func probeSource(f Family) Source {
	switch f {
	case FamilyURL:
		return Source{Kind: SourceURL, URL: "https://example.com"}
	case FamilyText:
		return Source{Kind: SourceFile, MimeType: "text/plain"}
	case FamilyDOCX:
		return Source{Kind: SourceFile, MimeType: MimeDOCX}
	case FamilyPDF:
		return Source{Kind: SourceFile, MimeType: MimePDF}
	case FamilyImage:
		return Source{Kind: SourceFile, MimeType: "image/png"}
	default:
		return Source{}
	}
}

// ProviderCapabilities is one provider's probed family support.
type ProviderCapabilities struct {
	Name     string
	Families map[Family]bool
}

// Capabilities probes every known provider — native, firecrawl, llamaparse —
// against one probeSource per Family, through the exact same Supports()
// method Submit's own precheck (internal/kbimport/enqueue.go) and
// internal/httpapi's GET /kb/import/providers both call. Each provider is
// constructed with an empty key and a nil *http.Client purely to probe:
// Supports() is a pure function of Source.Kind/MimeType, it never touches
// credentials or makes a network call (see each provider's own Supports
// doc comment). This makes the capability matrix data-derived from the
// real code rather than a second hand-maintained table that could drift —
// capabilities_test.go asserts the result matches plan/playground.md's own
// documented table.
func Capabilities() []ProviderCapabilities {
	providers := []Provider{
		NewNative(nil, 0),
		NewFirecrawl("", "", nil),
		NewLlamaParse("", "", nil),
	}
	out := make([]ProviderCapabilities, 0, len(providers))
	for _, p := range providers {
		fam := make(map[Family]bool, len(Families))
		for _, f := range Families {
			fam[f] = p.Supports(probeSource(f))
		}
		out = append(out, ProviderCapabilities{Name: p.Name(), Families: fam})
	}
	return out
}
