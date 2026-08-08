package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// kb_media_payload.go holds the wire-shape helpers every draft-lane req
// struct (topicReq/tariffReq/productReq/contactsReq, playground.go) needs to
// carry media edits: a nullable singular reference needs three states
// (absent/null/uuid) that a plain *uuid.UUID cannot distinguish, and every
// plural reference needs the same de-dupe + cap protection appendUnique
// (mcp_attach.go) already gives the attach path one id at a time.

// optionalUUID distinguishes an absent JSON key (Set=false — no pending
// edit) from an explicit `null` (Set=true, Value=nil — clear the field) and
// an explicit uuid string (Set=true, Value=&id): encoding/json alone
// collapses "absent" and "null" into the same Go zero value for a plain
// *uuid.UUID field, which is exactly the distinction featured_image's
// tri-state write needs (kbstore's **uuid.UUID contract: outer nil = leave,
// non-nil-pointing-at-nil = clear). UnmarshalJSON only ever runs when
// ShouldBindJSON actually finds this key in the request body — encoding/json
// never calls it for a key that's simply missing — so Set starts false and
// only ever flips to true from inside it.
type optionalUUID struct {
	Set   bool
	Value *uuid.UUID
}

func (o *optionalUUID) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}
	var id uuid.UUID
	if err := json.Unmarshal(data, &id); err != nil {
		return err
	}
	o.Value = &id
	return nil
}

// ptr converts the tri-state wire value into the **uuid.UUID a kbstore
// *Media field expects: absent (o.Set false) becomes a nil outer pointer —
// "no pending edit," so a media-blind client can never blank an
// MCP-authored reference. Present with a nil Value (explicit JSON null)
// becomes a non-nil pointer to a nil *uuid.UUID — "clear this field."
// Present with a Value becomes a non-nil pointer to it — "set this field."
func (o optionalUUID) ptr() **uuid.UUID {
	if !o.Set {
		return nil
	}
	v := o.Value
	return &v
}

// maxMediaIDsPerField bounds a single plural media field's incoming id
// count — the HTTP layer's own backstop against an unbounded request body;
// nothing upstream of validateMediaList caps this.
const maxMediaIDsPerField = 50

// validateMediaList caps and de-dupes a plural media field from the wire. A
// nil ids (the field was absent from the request body) is untouched and
// passed straight through as "no pending edit." An over-cap list fails the
// request with a 400 naming the field and returns ok=false — the caller
// must return immediately without any further response write.
//
// appendUnique (mcp_attach.go) gives the attach path this same de-dupe
// protection one id at a time; the whole-slice replace path here has no
// upstream equivalent, so it must do both itself.
func (s *Server) validateMediaList(c *gin.Context, field string, ids *[]uuid.UUID) (*[]uuid.UUID, bool) {
	if ids == nil {
		return nil, true
	}
	if len(*ids) > maxMediaIDsPerField {
		fail(c, http.StatusBadRequest, ErrValidation,
			fmt.Sprintf("%s: at most %d media references allowed, got %d", field, maxMediaIDsPerField, len(*ids)))
		return nil, false
	}
	out := make([]uuid.UUID, 0, len(*ids))
	seen := make(map[uuid.UUID]bool, len(*ids))
	for _, id := range *ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return &out, true
}
