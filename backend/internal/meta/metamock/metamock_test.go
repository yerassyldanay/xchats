package metamock

import (
	"context"
	"sync"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/messengerish"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
)

// These tests drive the mock through the REAL whatsappcloud.Client and
// messengerish.Client — the whole point of metamock is that those packages'
// own business logic (marshaling sendBody, unmarshaling SendResult, ...)
// runs completely unmodified against it, so a test that only called
// metamock's own methods directly would prove nothing about wire
// compatibility.

func TestWhatsAppCloud_PhoneNumbersAndRegister(t *testing.T) {
	c := whatsappcloud.NewClient(New())
	nums, err := c.PhoneNumbers(context.Background(), "waba-1", "token")
	if err != nil {
		t.Fatalf("PhoneNumbers: %v", err)
	}
	if len(nums) != 1 || nums[0].ID == "" || nums[0].DisplayPhoneNumber == "" {
		t.Fatalf("expected one plausible phone number, got %+v", nums)
	}
	if err := c.Register(context.Background(), nums[0].ID, "123456", "token"); err != nil {
		t.Fatalf("Register: %v", err)
	}
}

func TestWhatsAppCloud_OverrideRoundTrips(t *testing.T) {
	c := whatsappcloud.NewClient(New())
	ctx := context.Background()
	if uri, err := c.GetOverride(ctx, "waba-1", "token"); err != nil || uri != "" {
		t.Fatalf("GetOverride before any Set: uri=%q err=%v, want empty/nil", uri, err)
	}
	if err := c.SetOverride(ctx, "waba-1", whatsappcloud.SubscribedAppsOverride{
		CallbackURI: "https://example.test/webhook", VerifyToken: "vt",
	}, "token"); err != nil {
		t.Fatalf("SetOverride: %v", err)
	}
	if uri, err := c.GetOverride(ctx, "waba-1", "token"); err != nil || uri != "https://example.test/webhook" {
		t.Fatalf("GetOverride after Set: uri=%q err=%v, want the just-set callback", uri, err)
	}
	// A different WABA's override is independent state.
	if uri, err := c.GetOverride(ctx, "waba-2", "token"); err != nil || uri != "" {
		t.Fatalf("GetOverride for an untouched WABA: uri=%q err=%v, want empty", uri, err)
	}
	if err := c.ClearOverride(ctx, "waba-1", "token"); err != nil {
		t.Fatalf("ClearOverride: %v", err)
	}
	if uri, err := c.GetOverride(ctx, "waba-1", "token"); err != nil || uri != "" {
		t.Fatalf("GetOverride after Clear: uri=%q err=%v, want empty", uri, err)
	}
}

func TestWhatsAppCloud_SendTextAndMediaReturnUniqueIDs(t *testing.T) {
	c := whatsappcloud.NewClient(New())
	ctx := context.Background()
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		res, err := c.SendText(ctx, "phone-1", "+15550101", "hello", "token")
		if err != nil {
			t.Fatalf("SendText: %v", err)
		}
		if len(res.Messages) != 1 || res.Messages[0].ID == "" {
			t.Fatalf("expected one message id, got %+v", res)
		}
		if seen[res.Messages[0].ID] {
			t.Fatalf("duplicate provider message id %q across sends", res.Messages[0].ID)
		}
		seen[res.Messages[0].ID] = true
	}
	media, err := c.SendMedia(ctx, "phone-1", "+15550101", whatsappcloud.MediaImage, "https://example.test/img.png", "caption", "", "token")
	if err != nil || len(media.Messages) != 1 || media.Messages[0].ID == "" {
		t.Fatalf("SendMedia: res=%+v err=%v", media, err)
	}
}

func TestWhatsAppCloud_MediaInfoAndDownload(t *testing.T) {
	c := whatsappcloud.NewClient(New())
	ctx := context.Background()
	info, err := c.MediaInfo(ctx, "media-123", "token")
	if err != nil || info.URL == "" || info.MimeType == "" || info.FileSize == 0 {
		t.Fatalf("MediaInfo: info=%+v err=%v", info, err)
	}
	data, mime, err := c.DownloadMedia(ctx, info.URL, "token")
	if err != nil || len(data) == 0 || mime == "" {
		t.Fatalf("DownloadMedia: len=%d mime=%q err=%v", len(data), mime, err)
	}
}

func TestMessengerish_ProfileMeSubscribeSend(t *testing.T) {
	c := messengerish.NewClient(New())
	ctx := context.Background()

	prof, err := c.Profile(ctx, true, "ig-user-1", "token")
	if err != nil || prof.Name == "" {
		t.Fatalf("Profile: prof=%+v err=%v", prof, err)
	}
	me, err := c.Me(ctx, true, "token")
	if err != nil || me.UserID == "" {
		t.Fatalf("Me: me=%+v err=%v", me, err)
	}
	if err := c.Subscribe(ctx, true, "account-1", "token"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := c.Unsubscribe(ctx, true, "account-1", "token"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}

	res, err := c.SendText(ctx, true, "account-1", "psid-1", "hi there", "token")
	if err != nil || res.MessageID == "" {
		t.Fatalf("SendText: res=%+v err=%v", res, err)
	}
	res2, err := c.SendText(ctx, true, "account-1", "psid-1", "hi again", "token")
	if err != nil || res2.MessageID == res.MessageID {
		t.Fatalf("expected a different message id on the second send, got %q twice", res.MessageID)
	}

	media, err := c.SendMedia(ctx, false, "page-1", "psid-2", messengerish.MediaImage, "https://example.test/img.png", "token")
	if err != nil || media.RecipientID != "psid-2" || media.MessageID == "" {
		t.Fatalf("SendMedia: res=%+v err=%v", media, err)
	}
}

func TestOAuthMethodsReturnRealisticValues(t *testing.T) {
	c := New()
	ctx := context.Background()

	igShort, err := c.ExchangeInstagramCode(ctx, "app", "secret", "https://redirect", "code")
	if err != nil || igShort.AccessToken == "" || igShort.UserID == "" {
		t.Fatalf("ExchangeInstagramCode: %+v err=%v", igShort, err)
	}
	igLong, err := c.ExchangeInstagramLongLived(ctx, "secret", igShort.AccessToken)
	if err != nil || igLong.AccessToken == "" || igLong.ExpiresIn == 0 {
		t.Fatalf("ExchangeInstagramLongLived: %+v err=%v", igLong, err)
	}
	igRefreshed, err := c.RefreshInstagramLongLived(ctx, igLong.AccessToken)
	if err != nil || igRefreshed.AccessToken == "" {
		t.Fatalf("RefreshInstagramLongLived: %+v err=%v", igRefreshed, err)
	}

	fbShort, err := c.ExchangeFacebookCode(ctx, "app", "secret", "https://redirect", "code")
	if err != nil || fbShort.AccessToken == "" {
		t.Fatalf("ExchangeFacebookCode: %+v err=%v", fbShort, err)
	}
	fbLong, err := c.ExchangeFacebookLongLived(ctx, "app", "secret", fbShort.AccessToken)
	if err != nil || fbLong.AccessToken == "" {
		t.Fatalf("ExchangeFacebookLongLived: %+v err=%v", fbLong, err)
	}
	pages, err := c.PageAccounts(ctx, fbLong.AccessToken)
	if err != nil || len(pages) == 0 || pages[0].AccessToken == "" {
		t.Fatalf("PageAccounts: %+v err=%v", pages, err)
	}
	dbg, err := c.DebugToken(ctx, "input-token", "app-token")
	if err != nil || !dbg.IsValid || len(dbg.GranularScopes) == 0 {
		t.Fatalf("DebugToken: %+v err=%v", dbg, err)
	}

	if c.FacebookAuthorizeURL("app", "https://redirect", "state", []string{"scope1"}) == "" {
		t.Fatal("FacebookAuthorizeURL returned empty")
	}
	if c.EmbeddedSignupAuthorizeURL("app", "config", "https://redirect", "state", "") == "" {
		t.Fatal("EmbeddedSignupAuthorizeURL returned empty")
	}
}

// TestConcurrentSendsAreRaceFree exercises the mock under the -race detector
// with concurrent callers — the shape of the load harness's own usage (many
// worker goroutines sending through the same shared client).
func TestConcurrentSendsAreRaceFree(t *testing.T) {
	c := whatsappcloud.NewClient(New())
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.SendText(ctx, "phone-1", "+15550101", "hello", "token"); err != nil {
				t.Errorf("SendText: %v", err)
			}
		}()
	}
	wg.Wait()
}

// TestCallHistoryIsBounded guards maxRecordedCalls — a long profiling run
// must not grow this mock's own memory without bound.
func TestCallHistoryIsBounded(t *testing.T) {
	c := New()
	ctx := context.Background()
	for i := 0; i < maxRecordedCalls*3; i++ {
		_, _ = c.PageAccounts(ctx, "token")
	}
	if got := len(c.Calls()); got != maxRecordedCalls {
		t.Fatalf("Calls() len = %d, want bounded to %d", got, maxRecordedCalls)
	}
}
