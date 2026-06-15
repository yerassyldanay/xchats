package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleRealtime is the SSE stream. It hydrates nothing (the panes GET on mount);
// it is purely the live delta layer: message.*, chat.*, ai_draft.*.
func (s *Server) handleRealtime(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		fail(c, http.StatusInternalServerError, ErrInternal, "streaming unsupported")
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	events, unsubscribe := s.hub.Subscribe()
	defer unsubscribe()

	// initial comment so proxies flush headers
	_, _ = c.Writer.WriteString(": connected\n\n")
	flusher.Flush()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, alive := <-events:
			if !alive {
				return
			}
			data, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			_, _ = c.Writer.WriteString("event: " + ev.Name + "\n")
			_, _ = c.Writer.WriteString("data: " + string(data) + "\n\n")
			flusher.Flush()
		}
	}
}
