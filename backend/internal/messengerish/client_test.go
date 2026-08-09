package messengerish

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/meta"
)

// testClient mirrors whatsappcloud's own testClient helper: instagramSrv
// answers every graph.instagram.com-routed call, facebookSrv every
// graph.facebook.com-routed one — a test only needs to set up the one its
// case actually exercises.
func testClient(t *testing.T, instagramHandler, facebookHandler http.HandlerFunc) *Client {
	t.Helper()
	var igURL, fbURL string
	if instagramHandler != nil {
		srv := httptest.NewServer(instagramHandler)
		t.Cleanup(srv.Close)
		igURL = srv.URL
	}
	if facebookHandler != nil {
		srv := httptest.NewServer(facebookHandler)
		t.Cleanup(srv.Close)
		fbURL = srv.URL
	}
	h := meta.NewHTTPWithHosts("v21.0", fbURL, igURL, "", slog.Default())
	return NewClient(h)
}

func TestProfileUsesInstagramHostWithUsername(t *testing.T) {
	var gotPath, gotAuth string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"name":"Клиент Тест","username":"client_test"}`))
	}, nil)
	p, err := c.Profile(context.Background(), true, "1234567890123456", "ig-token")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if gotPath != "/v21.0/1234567890123456" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer ig-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if p.Name != "Клиент Тест" || p.Username != "client_test" {
		t.Fatalf("profile = %+v", p)
	}
}

func TestProfileUsesFacebookHostWithoutUsername(t *testing.T) {
	var gotPath string
	c := testClient(t, nil, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"name":"Клиент Тест"}`))
	})
	p, err := c.Profile(context.Background(), false, "psid-1", "page-token")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if gotPath != "/v21.0/psid-1" {
		t.Fatalf("path = %q", gotPath)
	}
	if p.Name != "Клиент Тест" || p.Username != "" {
		t.Fatalf("profile = %+v", p)
	}
}

func TestMeResolvesTheConnectedAccountsOwnIdentity(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user_id":"17841400000000001","username":"my_shop","name":"My Shop"}`))
	}, nil)
	me, err := c.Me(context.Background(), true, "ig-token")
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.UserID != "17841400000000001" || me.Username != "my_shop" {
		t.Fatalf("me = %+v", me)
	}
}

func TestSubscribeSendsSubscribedFields(t *testing.T) {
	var gotPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		_, _ = w.Write([]byte(`{"success":true}`))
	}, nil)
	if err := c.Subscribe(context.Background(), true, "17841400000000001", "ig-token"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if gotPath != "/v21.0/17841400000000001/subscribed_apps?subscribed_fields=messages" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestSendTextBuildsTheDocumentedBody(t *testing.T) {
	var gotBody map[string]any
	var gotPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"recipient_id":"1234567890123456","message_id":"aWdfZAG1faXRlbToxOjIz"}`))
	}, nil)
	res, err := c.SendText(context.Background(), true, "17841400000000001", "1234567890123456", "привет", "ig-token")
	if err != nil {
		t.Fatalf("SendText: %v", err)
	}
	if gotPath != "/v21.0/17841400000000001/messages" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["messaging_type"] != "RESPONSE" {
		t.Fatalf("messaging_type = %v", gotBody["messaging_type"])
	}
	recipient, _ := gotBody["recipient"].(map[string]any)
	if recipient["id"] != "1234567890123456" {
		t.Fatalf("recipient = %v", recipient)
	}
	message, _ := gotBody["message"].(map[string]any)
	if message["text"] != "привет" {
		t.Fatalf("message.text = %v", message["text"])
	}
	if res.MessageID != "aWdfZAG1faXRlbToxOjIz" {
		t.Fatalf("result = %+v", res)
	}
}

func TestSendMediaPlacesLinkUnderAttachmentPayload(t *testing.T) {
	var gotBody map[string]any
	c := testClient(t, nil, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"recipient_id":"psid-1","message_id":"m1"}`))
	})
	_, err := c.SendMedia(context.Background(), false, "page-1", "psid-1", MediaImage,
		"https://xchats.example/meta/api/v1/media/abc?token=x&exp=1", "page-token")
	if err != nil {
		t.Fatalf("SendMedia: %v", err)
	}
	if _, hasText := gotBody["text"]; hasText {
		t.Fatal("a media send must not carry a top-level text field")
	}
	message, _ := gotBody["message"].(map[string]any)
	if _, hasText := message["text"]; hasText {
		t.Fatal("a media send's message must not carry a text field")
	}
	att, _ := message["attachment"].(map[string]any)
	if att["type"] != "image" {
		t.Fatalf("attachment.type = %v", att["type"])
	}
	payload, _ := att["payload"].(map[string]any)
	if payload["url"] != "https://xchats.example/meta/api/v1/media/abc?token=x&exp=1" {
		t.Fatalf("payload.url = %v", payload["url"])
	}
	if payload["is_reusable"] != false {
		t.Fatalf("payload.is_reusable = %v, want false", payload["is_reusable"])
	}
}
