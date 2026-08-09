package worker

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

const metaMediaTestEncKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// recordingQueue is a minimal queue.Queue fake that just records every
// Publish call — used where a test needs to inspect what was enqueued
// rather than actually run it through a worker pool.
type recordingQueue struct {
	published []queue.Message
}

func (q *recordingQueue) Publish(_ context.Context, m queue.Message) error {
	q.published = append(q.published, m)
	return nil
}
func (q *recordingQueue) Start(context.Context, queue.Handler) {}
func (q *recordingQueue) Wait()                                {}
func (q *recordingQueue) Close()                               {}

// seedWhatsAppCloudMediaFixture builds an account with stored credentials
// plus one inbound message with a pending attachment — the exact state
// downloadMetaMedia expects to find when its task fires.
func seedWhatsAppCloudMediaFixture(t *testing.T, st *store.Store, token string) (accountID, messageID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	box, err := secretbox.FromEnvValue(metaMediaTestEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)
	org, err := st.SeedOrganization(ctx, "worker-meta-media-test")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	acct, err := st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID: config.ChannelAccountID(config.WhatsAppCloudOwnerRef("pn-media-1")), OrganizationID: org.ID,
		Channel: "whatsapp_cloud", ExternalAccountID: "pn-media-1",
	})
	if err != nil {
		t.Fatalf("claim channel account: %v", err)
	}
	if err := st.SetChannelCredentials(ctx, store.ChannelCredentialsWrite{
		AccountID: acct.ID, Secret: token, TokenKind: "business_token",
	}); err != nil {
		t.Fatalf("set channel credentials: %v", err)
	}
	inbound, err := st.IngestChannelInbound(ctx, store.ChannelInbound{
		AccountID: acct.ID, ExternalContactID: "c1", ExternalThreadID: "c1",
		Direction: "in", SenderKind: "contact", ExternalMessageID: "wamid.media1", MessageKind: "imageMessage",
	})
	if err != nil {
		t.Fatalf("ingest channel inbound: %v", err)
	}
	if err := st.InsertChannelMediaPending(ctx, inbound.MessageID, store.ChannelMediaMeta{
		MediaType: "image", ProviderRef: "media-obj-1",
	}); err != nil {
		t.Fatalf("insert channel media pending: %v", err)
	}
	return acct.ID, inbound.MessageID
}

func TestHandleMediaDownloadDownloadsWhatsAppCloudAttachment(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	accountID, messageID := seedWhatsAppCloudMediaFixture(t, st, "biz-token-1")

	var gotAuthHeaders []string
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeaders = append(gotAuthHeaders, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpeg-bytes"))
	}))
	defer dlSrv.Close()
	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeaders = append(gotAuthHeaders, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"url":"` + dlSrv.URL + `","mime_type":"image/jpeg","id":"media-obj-1"}`))
	}))
	defer graphSrv.Close()

	waCloud := whatsappcloud.NewClient(meta.NewHTTPWithHosts("v21.0", graphSrv.URL, "", testLogger()))
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	w := &Worker{Store: st, WACloud: waCloud, Blob: blobStore, Hub: realtime.NewHub(), Log: testLogger()}

	task := MediaDownloadTask{
		MessageID: messageID, Channel: messaging.ChannelWhatsAppCloud, AccountID: accountID,
		FileID: "media-obj-1", BlobID: ChannelBlobID(messageID),
	}
	if err := w.handleMediaDownload(ctx, task); err != nil {
		t.Fatalf("handleMediaDownload: %v", err)
	}

	data, bmeta, err := blobStore.Get(ChannelBlobID(messageID))
	if err != nil {
		t.Fatalf("blob.Get: %v", err)
	}
	if string(data) != "jpeg-bytes" {
		t.Fatalf("blob data = %q", data)
	}
	if bmeta.Mimetype != "image/jpeg" {
		t.Fatalf("blob mimetype = %q", bmeta.Mimetype)
	}
	for _, h := range gotAuthHeaders {
		if h != "Bearer biz-token-1" {
			t.Fatalf("Authorization = %q, want the account's own resolved token on every Graph/download call", h)
		}
	}
	if len(gotAuthHeaders) != 2 {
		t.Fatalf("Graph calls made = %d, want 2 (MediaInfo + DownloadMedia)", len(gotAuthHeaders))
	}
}

func TestHandleMediaDownloadUnsupportedChannelIsANoop(t *testing.T) {
	st := dbtest.New(t)
	w := &Worker{Store: st, Hub: realtime.NewHub(), Log: testLogger()}
	err := w.handleMediaDownload(context.Background(), MediaDownloadTask{
		MessageID: uuid.New(), Channel: messaging.ChannelInstagram, AccountID: uuid.New(),
		FileID: "cdn-ref", BlobID: "some-blob-key",
	})
	if err != nil {
		t.Fatalf("handleMediaDownload for an unrouted channel = %v, want nil (logged, not fatal)", err)
	}
}

func TestSweepChannelMediaEnqueuesPendingAttachments(t *testing.T) {
	ctx := context.Background()
	// dbtest.Open (not New): the row is backdated directly below, the same
	// way store.TestPendingTelegramMediaFiltersByAge does — a row updated
	// moments ago must NOT match an age-filtered sweep (see
	// PendingChannelMedia's own doc comment), so a JUST-inserted row needs
	// its updated_at pushed into the past to exercise the "genuinely stale"
	// path this test is actually about.
	st, db := dbtest.Open(t)
	_, messageID := seedWhatsAppCloudMediaFixture(t, st, "biz-token-2")
	if _, err := db.Exec(ctx, `UPDATE channel_message_media SET updated_at = $2 WHERE message_id = $1`,
		messageID, time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatalf("backdate media: %v", err)
	}

	w := &Worker{Store: st, Hub: realtime.NewHub(), Log: testLogger()}
	pub := &recordingQueue{}
	w.Queue = pub

	n, err := w.SweepChannelMedia(ctx, time.Hour, 10)
	if err != nil {
		t.Fatalf("SweepChannelMedia: %v", err)
	}
	if n != 1 {
		t.Fatalf("queued = %d, want 1", n)
	}
	if len(pub.published) != 1 {
		t.Fatalf("published = %d, want 1", len(pub.published))
	}
	task, ok := pub.published[0].Payload.(MediaDownloadTask)
	if !ok {
		t.Fatalf("payload type = %T, want MediaDownloadTask", pub.published[0].Payload)
	}
	if task.MessageID != messageID || task.Channel != messaging.ChannelWhatsAppCloud || task.FileID != "media-obj-1" {
		t.Fatalf("task = %+v", task)
	}
}
