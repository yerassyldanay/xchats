package httpapi

import (
	"encoding/json"
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

// --- materials (Stage 1: drop any input → ai_materials + enqueue extraction) ---

type materialReq struct {
	SourceType string `json:"source_type"` // 'text' | 'url'
	Text       string `json:"text"`
	URL        string `json:"url"`
}

// handlePlaygroundCreateMaterial stages one dropped input. A file upload (multipart)
// becomes an image/pdf/doc/video material with its bytes stored; a JSON body carries
// text or a URL. Everything but plain text enqueues an extraction job.
func (s *Server) handlePlaygroundCreateMaterial(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if _, err := s.kb.OpenDraft(ctx(c), orgID); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
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
		in = kbstore.MaterialInput{SourceType: mediaType, SourceRef: fh.Filename, BlobID: blobID, MediaKind: mediaType}
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
			in = kbstore.MaterialInput{SourceType: "url", SourceRef: strings.TrimSpace(req.URL)}
		default:
			fail(c, http.StatusBadRequest, ErrValidation, "file, text or url required")
			return
		}
	}

	m, err := s.kb.CreateMaterial(ctx(c), orgID, in)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
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
	view, err := s.kb.GetDraft(ctx(c), orgID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
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
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	view, err := s.kb.GetDraft(ctx(c), orgID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
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
	view, err := s.kb.GetDraft(ctx(c), orgID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
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
// describe_media → fill an asset description or a material's text.
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
	if err := s.kb.ResolveRequest(ctx(c), reqID, orDefaultStr(body.State, "resolved"), string(orDefaultJSON(body.Resolution))); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.kbChanged(c, orgID)
}

// applyResolution turns a popup answer into a draft mutation, keyed by req_type.
func (s *Server) applyResolution(c *gin.Context, orgID uuid.UUID, r kbstore.Request, resolution json.RawMessage) error {
	target := decodeObj(r.Target)
	answer := decodeObj(string(resolution))

	switch r.ReqType {
	case "confirm_value":
		token, _ := target["token"].(string)
		lang, _ := target["lang"].(string)
		valueText, _ := answer["value_text"].(string)
		if valueText == "" {
			// Accept the detected value as-is (the suggestion lives on the request).
			valueText, _ = decodeObj(r.Context)["suggested"].(string)
		}
		if token == "" || valueText == "" {
			return errString2("token and value_text required")
		}
		// A human-confirmed value is approved (it passed the popup).
		return s.kb.UpsertValue(ctx(c), orgID, kbstore.ValueInput{
			Token: token, Lang: orDefaultStr(lang, "ru"), ValueText: valueText,
			ReviewState: "approved", Provenance: `{"source":"confirm_value"}`,
		})
	case "describe_media":
		desc, _ := answer["description"].(string)
		if ref, _ := target["asset_ref"].(string); ref != "" {
			return s.kb.PatchAsset(ctx(c), orgID, ref, kbstore.AssetPatch{Description: &desc})
		}
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
