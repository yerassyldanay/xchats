package whatsappcloud

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/meta"
)

func testClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	h := meta.NewHTTPWithHosts("v21.0", srv.URL, "", slog.Default())
	return NewClient(h), srv
}

func TestPhoneNumbers(t *testing.T) {
	var gotPath, gotAuth string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"pn1","display_phone_number":"+1 555 000 1111","verified_name":"Acme","quality_rating":"GREEN"}]}`))
	})
	nums, err := c.PhoneNumbers(context.Background(), "waba-1", "biz-token")
	if err != nil {
		t.Fatalf("PhoneNumbers: %v", err)
	}
	if gotPath != "/v21.0/waba-1/phone_numbers" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer biz-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if len(nums) != 1 || nums[0].ID != "pn1" || nums[0].QualityRating != "GREEN" {
		t.Fatalf("nums = %+v", nums)
	}
}

func TestRegisterSendsPIN(t *testing.T) {
	var gotBody map[string]string
	var gotPath string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	if err := c.Register(context.Background(), "pn1", "123456", "biz-token"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if gotPath != "/v21.0/pn1/register" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["messaging_product"] != "whatsapp" || gotBody["pin"] != "123456" {
		t.Fatalf("body = %+v", gotBody)
	}
}

func TestSetOverrideSendsCallbackAndVerifyToken(t *testing.T) {
	var gotBody SubscribedAppsOverride
	var gotPath string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	err := c.SetOverride(context.Background(), "waba-1", SubscribedAppsOverride{
		CallbackURI: "https://tunnel.example/meta/api/v1/webhook/whatsapp",
		VerifyToken: "verify-tok",
	}, "biz-token")
	if err != nil {
		t.Fatalf("SetOverride: %v", err)
	}
	if gotPath != "/v21.0/waba-1/subscribed_apps" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody.CallbackURI != "https://tunnel.example/meta/api/v1/webhook/whatsapp" || gotBody.VerifyToken != "verify-tok" {
		t.Fatalf("body = %+v", gotBody)
	}
}

func TestGetOverrideReadsBackCurrentCallback(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"data":[{"whatsapp_business_api_data":{"id":"app1"},"override_callback_uri":"https://old-tunnel.example/hook"}]}`))
	})
	got, err := c.GetOverride(context.Background(), "waba-1", "biz-token")
	if err != nil {
		t.Fatalf("GetOverride: %v", err)
	}
	if got != "https://old-tunnel.example/hook" {
		t.Fatalf("got = %q", got)
	}
}

func TestGetOverrideEmptyWhenNoneRegistered(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	got, err := c.GetOverride(context.Background(), "waba-1", "biz-token")
	if err != nil {
		t.Fatalf("GetOverride: %v", err)
	}
	if got != "" {
		t.Fatalf("got = %q, want empty", got)
	}
}

func TestClearOverridePostsEmptyBody(t *testing.T) {
	var gotBody SubscribedAppsOverride
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	if err := c.ClearOverride(context.Background(), "waba-1", "biz-token"); err != nil {
		t.Fatalf("ClearOverride: %v", err)
	}
	if gotBody.CallbackURI != "" || gotBody.VerifyToken != "" {
		t.Fatalf("body = %+v, want both empty", gotBody)
	}
}

func TestSendTextBuildsTheDocumentedBody(t *testing.T) {
	var gotBody map[string]any
	var gotPath string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.abc123"}]}`))
	})
	res, err := c.SendText(context.Background(), "pn1", "77011234567", "привет", "biz-token")
	if err != nil {
		t.Fatalf("SendText: %v", err)
	}
	if gotPath != "/v21.0/pn1/messages" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["messaging_product"] != "whatsapp" || gotBody["recipient_type"] != "individual" ||
		gotBody["to"] != "77011234567" || gotBody["type"] != "text" {
		t.Fatalf("body = %+v", gotBody)
	}
	text, _ := gotBody["text"].(map[string]any)
	if text["body"] != "привет" {
		t.Fatalf("text.body = %v", text["body"])
	}
	if len(res.Messages) != 1 || res.Messages[0].ID != "wamid.abc123" {
		t.Fatalf("result = %+v", res)
	}
}

func TestSendMediaImagePlacesLinkUnderTheImageKey(t *testing.T) {
	var gotBody map[string]any
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.def456"}]}`))
	})
	_, err := c.SendMedia(context.Background(), "pn1", "77011234567", MediaImage,
		"https://xchats.example/meta/api/v1/media/abc?token=x&exp=1", "caption text", "", "biz-token")
	if err != nil {
		t.Fatalf("SendMedia: %v", err)
	}
	if gotBody["type"] != "image" {
		t.Fatalf("type = %v", gotBody["type"])
	}
	if _, hasText := gotBody["text"]; hasText {
		t.Fatal("a media send must not carry a text field")
	}
	img, _ := gotBody["image"].(map[string]any)
	if img["link"] != "https://xchats.example/meta/api/v1/media/abc?token=x&exp=1" || img["caption"] != "caption text" {
		t.Fatalf("image = %+v", img)
	}
	if _, hasFilename := img["filename"]; hasFilename {
		t.Fatal("filename must only be set for document sends")
	}
}

func TestSendMediaDocumentIncludesFilename(t *testing.T) {
	var gotBody map[string]any
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.ghi789"}]}`))
	})
	_, err := c.SendMedia(context.Background(), "pn1", "77011234567", MediaDocument,
		"https://xchats.example/meta/api/v1/media/xyz?token=y&exp=2", "", "invoice.pdf", "biz-token")
	if err != nil {
		t.Fatalf("SendMedia: %v", err)
	}
	doc, _ := gotBody["document"].(map[string]any)
	if doc["filename"] != "invoice.pdf" {
		t.Fatalf("document.filename = %v", doc["filename"])
	}
}

func TestMediaInfoAndDownloadRequireTheSameBearerToken(t *testing.T) {
	var downloadAuth string
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloadAuth = r.Header.Get("Authorization")
		if downloadAuth != "Bearer biz-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("bytes"))
	}))
	t.Cleanup(dlSrv.Close)

	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"url":"` + dlSrv.URL + `","mime_type":"image/jpeg","sha256":"abc","file_size":5,"id":"media-1"}`))
	})
	info, err := c.MediaInfo(context.Background(), "media-1", "biz-token")
	if err != nil {
		t.Fatalf("MediaInfo: %v", err)
	}
	if info.URL != dlSrv.URL {
		t.Fatalf("info.URL = %q", info.URL)
	}
	body, ct, err := c.DownloadMedia(context.Background(), info.URL, "biz-token")
	if err != nil {
		t.Fatalf("DownloadMedia: %v", err)
	}
	if string(body) != "bytes" || ct != "image/jpeg" {
		t.Fatalf("body/ct = %q, %q", body, ct)
	}
}

func TestMediaDownloadFailsWithoutTheBearerToken(t *testing.T) {
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("bytes"))
	}))
	t.Cleanup(dlSrv.Close)
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {})
	if _, _, err := c.DownloadMedia(context.Background(), dlSrv.URL, ""); err == nil {
		t.Fatal("expected an error when downloading without a bearer token")
	}
}
