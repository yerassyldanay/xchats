package blob

import (
	"fmt"
	"net/http"
	"strings"
)

// MimeSanityCheck is a lightweight guard against a renamed/mislabeled
// upload, not a full content-type authority: for image/video/audio the
// sniffed top-level type must agree with what was declared (these are the
// types a widget will render inline), and a payload that sniffs as HTML is
// always rejected unless HTML was actually declared (the classic
// stored-XSS-via-mislabeled-upload vector) — everything else (documents:
// application/*, text/* other than html) is accepted as declared, since
// office/PDF formats sniff too inconsistently to cross-check usefully.
//
// Moved here from internal/httpapi (formerly the unexported
// mimeSanityCheck in mcp_upload.go) so internal/kbimport — which never
// imports internal/httpapi — can apply the exact same check to a
// downloaded embedded image before staging it as a material.
func MimeSanityCheck(declared string, data []byte) string {
	sniffed := http.DetectContentType(data)
	sniffedTop := strings.SplitN(strings.SplitN(sniffed, ";", 2)[0], "/", 2)[0]
	declaredTop := strings.SplitN(declared, "/", 2)[0]
	if (declaredTop == "image" || declaredTop == "video" || declaredTop == "audio") && sniffedTop != declaredTop {
		return fmt.Sprintf("declared mime_type %q does not match the uploaded content (looks like %s)", declared, sniffed)
	}
	if strings.HasPrefix(sniffed, "text/html") && declaredTop != "text" {
		return "uploaded content looks like HTML, which is not an accepted KB media type"
	}
	return ""
}
