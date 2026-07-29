package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
)

// --- materials (Stage 1: drop any input → kbd_materials + enqueue extraction) ---

type materialReq struct {
	SourceType  string `json:"source_type"` // 'text' | 'url'
	Text        string `json:"text"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// handlePlaygroundCreateMaterial stages one dropped input. A file upload (multipart)
// becomes an image/pdf/doc/video material with its bytes stored; a JSON body carries
// text or a URL. An optional operator "description" (multipart form field, or JSON
// body field for a URL) is the parsing comment — see MaterialInput.Description; a
// described file/media material is born ready and skips the extraction job.
func (s *Server) handlePlaygroundCreateMaterial(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}

	var in kbstore.MaterialInput
	if fh, ferr := c.FormFile("file"); ferr == nil {
		f, err := fh.Open()
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "cannot open file")
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "cannot read file")
			return
		}
		mediaType, mimetype := detectMedia(fh.Filename, fh.Header.Get("Content-Type"))
		blobID := uuid.NewString()
		if _, err := s.blob.Put(blobID, data, blob.Meta{MediaType: mediaType, Mimetype: mimetype, FileName: fh.Filename, FileSize: int64(len(data))}); err != nil {
			fail(c, http.StatusBadGateway, ErrMediaUnavailable, "store failed")
			return
		}
		in = kbstore.MaterialInput{SourceType: mediaType, SourceRef: fh.Filename, BlobID: blobID, MediaKind: mediaType,
			Description: c.PostForm("description")}
	} else {
		var req materialReq
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "file, text or url required")
			return
		}
		switch {
		case strings.TrimSpace(req.Text) != "":
			in = kbstore.MaterialInput{SourceType: "text", SourceRef: "chat", Text: req.Text}
		case strings.TrimSpace(req.URL) != "":
			in = kbstore.MaterialInput{SourceType: "url", SourceRef: strings.TrimSpace(req.URL), Description: req.Description}
		default:
			fail(c, http.StatusBadRequest, ErrValidation, "file, text or url required")
			return
		}
	}

	m, err := s.kb.CreateMaterial(ctx(c), orgID, in)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	// Text is born ready; everything else needs an extraction pass.
	if m.Status != "ready" {
		_ = s.queue.Publish(queue.Message{Kind: queue.KindExtractMaterial,
			Payload: worker.ExtractMaterialTask{MaterialID: m.ID, OrgID: orgID}})
	}
	s.hub.Broadcast("kb.material.updated", gin.H{"material_id": m.ID, "status": m.Status})
	created(c, m)
}

func (s *Server) handlePlaygroundListMaterials(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	view, err := s.kb.Draft(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, gin.H{"items": view.Materials})
}

// --- builder chat (Stage 2: synthesize ready materials into the draft) ---

type chatReq struct {
	Instruction string `json:"instruction"`
}

func (s *Server) handlePlaygroundChat(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if s.builder == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "builder not available")
		return
	}
	var req chatReq
	_ = c.ShouldBindJSON(&req)
	res, err := s.builder.RunTurn(ctx(c), s.kb, orgID, req.Instruction)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	view, err := s.kb.Draft(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, gin.H{"result": res, "draft": view})
}

// --- requests (popups) -----------------------------------------------------

func (s *Server) handlePlaygroundListRequests(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	view, err := s.kb.Draft(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, gin.H{"items": view.Requests})
}

type resolveReq struct {
	State      string          `json:"state"` // 'resolved' (default) | 'dismissed'
	Resolution json.RawMessage `json:"resolution"`
}

// handlePlaygroundResolveRequest applies the operator's popup answer to the draft,
// then marks the request resolved. confirm_value → upsert the (confirmed) value;
// describe_file → fill a material's text.
func (s *Server) handlePlaygroundResolveRequest(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	reqID, proceed := parseUUID(c, "id")
	if !proceed {
		return
	}
	var body resolveReq
	_ = c.ShouldBindJSON(&body)

	r, err := s.kb.GetRequest(ctx(c), reqID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "request not found")
		return
	}

	if body.State != "dismissed" {
		if err := s.applyResolution(c, orgID, r, body.Resolution); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, err.Error())
			return
		}
	}
	state := orDefaultStr(body.State, "resolved")
	if err := s.kb.ResolveRequest(ctx(c), reqID, state, string(orDefaultJSON(body.Resolution))); err != nil {
		s.kbFail(c, err)
		return
	}
	s.hub.Broadcast("kb.request.resolved", gin.H{"id": reqID, "state": state})
	s.kbChanged(c, orgID)
}

// applyResolution turns a popup answer into a draft mutation, keyed by req_type.
func (s *Server) applyResolution(c *gin.Context, orgID uuid.UUID, r kbstore.Request, resolution json.RawMessage) error {
	target := decodeObj(r.Target)
	answer := decodeObj(string(resolution))

	switch r.ReqType {
	case "confirm_fact":
		// The operator confirmed a detected fact → it lands as a pending typed-fact
		// blob entry (concrete column); approving materializes it into the live table.
		// target = {table, slug, field}; answer = {value} (falls back to context.suggested).
		table, _ := target["table"].(string)
		slug, _ := target["slug"].(string)
		field, _ := target["field"].(string)
		value, _ := answer["value"].(string)
		if value == "" {
			value, _ = decodeObj(r.Context)["suggested"].(string)
		}
		if table == "" || slug == "" || field == "" || value == "" {
			return errString2("table, slug, field and value required")
		}
		return s.upsertFactField(c, orgID, table, slug, field, value)
	case "describe_file":
		desc, _ := answer["description"].(string)
		if mid, _ := target["material_id"].(string); mid != "" {
			id, perr := uuid.Parse(mid)
			if perr != nil {
				return errString2("bad material_id")
			}
			return s.kb.UpdateMaterialExtraction(ctx(c), id, kbstore.MaterialExtraction{
				Status: "ready", ExtractedText: desc, Extraction: `{"method":"operator"}`,
			})
		}
	}
	return nil // comment / unknown → no mutation, just mark resolved
}

// upsertFactField writes one confirmed fact column onto its typed entity (pending
// in the draft blob until approve).
func (s *Server) upsertFactField(c *gin.Context, orgID uuid.UUID, table, slug, field, value string) error {
	if err := s.kb.SetFactField(ctx(c), orgID, table, slug, field, value); err != nil {
		if errors.Is(err, kbstore.ErrUnknownKind) {
			return errString2("unknown fact table/field")
		}
		return err
	}
	return nil
}

// --- small helpers ---------------------------------------------------------

func decodeObj(raw string) map[string]any {
	m := map[string]any{}
	if raw == "" {
		return m
	}
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}

func orDefaultStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func orDefaultJSON(j json.RawMessage) json.RawMessage {
	if len(j) == 0 {
		return json.RawMessage("{}")
	}
	return j
}

type errString2 string

func (e errString2) Error() string { return string(e) }
