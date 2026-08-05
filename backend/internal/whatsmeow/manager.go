// Package whatsmeow is the go.mau.fi/whatsmeow adapter — the SOLE importer
// of that library in this module. Everything above this package (httpapi,
// worker, main.go) depends only on the provider-neutral internal/whatsapp
// package; this package translates between the two.
package whatsmeow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	wm "go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	wmstore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// mediaDownloadTimeout bounds one attachment's background download — well
// past any reasonable WhatsApp media size, short enough that a stuck
// download doesn't accumulate goroutines under sustained media traffic.
const mediaDownloadTimeout = 2 * time.Minute

// aiDraftEnqueueTimeout bounds the fire-and-forget AI-draft enqueue — a full
// queue becomes a logged error rather than blocking event ingestion.
const aiDraftEnqueueTimeout = 2 * time.Second

// ManagerConfig is Manager's construction input.
type ManagerConfig struct {
	// DeviceDBPath is whatsmeow's own device-session SQLite file — separate
	// from xchats.db (see store.go's doc comment).
	DeviceDBPath string
	Store        *store.Store
	Blob         blob.Store
	Queue        queue.Queue
	// Hub broadcasts message.created/chat.updated (and friends) as each event
	// is ingested — without it neither a real inbound message nor a
	// /debug/wa-event injection would ever reach the inbox UI live.
	Hub *realtime.Hub
	Log *slog.Logger
}

// Manager is the whatsmeow adapter's composition root: a registry of
// per-account managed clients, plus the account lifecycle (pair/logout/
// status/debug-inject) httpapi drives through the whatsapp.Manager port.
type Manager struct {
	cfg       ManagerConfig
	container *sqlstore.Container
	log       *slog.Logger

	mu      sync.Mutex
	clients map[string]*managedClient // accountID (string uuid) -> live client

	pairings *pairingRegistry

	// newClient constructs the waClient for a device — wm.NewClient in
	// production. Tests substitute a fake here (unexported field, same
	// package) so Pair/reconnect never have to dial a real WhatsApp
	// connection to be exercised.
	newClient func(device *wmstore.Device, log waLog.Logger) waClient
}

var _ whatsapp.Manager = (*Manager)(nil)

// NewManager opens whatsmeow's own device database and returns a Manager
// ready for Start. It does not connect any account yet.
func NewManager(ctx context.Context, cfg ManagerConfig) (*Manager, error) {
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	container, err := openDeviceStore(ctx, cfg.DeviceDBPath, NewLogger(log, "whatsmeow-store"))
	if err != nil {
		return nil, err
	}
	return &Manager{
		cfg: cfg, container: container, log: log,
		clients:  make(map[string]*managedClient),
		pairings: newPairingRegistry(),
		newClient: func(device *wmstore.Device, log waLog.Logger) waClient {
			return wm.NewClient(device, log)
		},
	}, nil
}

// Close disconnects every managed client and closes the device database.
func (m *Manager) Close() {
	m.mu.Lock()
	clients := make([]*managedClient, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.clients = map[string]*managedClient{}
	m.mu.Unlock()
	for _, c := range clients {
		c.stop()
		c.client.Disconnect()
	}
	_ = m.container.Close()
}

// Start reconnects every saved account — the boot-time pass that restores
// live WhatsApp connections without re-scanning a QR code. Best-effort: one
// account's reconnect failure is logged and does not stop the others.
func (m *Manager) Start(ctx context.Context) {
	creds, err := m.cfg.Store.ListWaCredentials(ctx)
	if err != nil {
		m.log.Error("whatsmeow: list saved accounts failed", "err", err)
		return
	}
	for _, c := range creds {
		if err := m.reconnect(ctx, c.AccountID, c.DeviceJID); err != nil {
			m.log.Error("whatsmeow: reconnect failed", "account_id", c.AccountID, "err", err)
		}
	}
}

func (m *Manager) reconnect(ctx context.Context, accountID uuid.UUID, deviceJID string) error {
	jid, err := types.ParseJID(deviceJID)
	if err != nil {
		return fmt.Errorf("parse saved device jid %q: %w", deviceJID, err)
	}
	device, err := m.container.GetDevice(ctx, jid)
	if err != nil {
		return fmt.Errorf("load device: %w", err)
	}
	if device == nil {
		return fmt.Errorf("no saved session for device %q", deviceJID)
	}
	cli := m.newClient(device, NewLogger(m.log, "whatsmeow-"+accountID.String()))
	m.registerClient(accountID.String(), cli)
	if err := cli.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return nil
}

// registerClient wraps cli in a managedClient wired to handleEvent, and adds
// it to the registry (replacing — and stopping — any prior entry for the
// same account, e.g. a reconnect after a stale registration).
func (m *Manager) registerClient(accountID string, cli waClient) *managedClient {
	mc := newManagedClient(accountID, cli, m, m.log)
	m.mu.Lock()
	old, hadOld := m.clients[accountID]
	m.clients[accountID] = mc
	m.mu.Unlock()
	if hadOld {
		old.stop()
	}
	return mc
}

func (m *Manager) clientFor(accountID string) (*managedClient, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.clients[accountID]
	return mc, ok
}

func (m *Manager) removeClient(accountID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, accountID)
}

// ---------------------------------------------------------------------------
// Event dispatch — the func(any) every managed client's whatsmeow.Client
// calls (via managedClient's backpressure queue). Real events and
// /debug/wa-event injections converge on the SAME ingestMessage/
// applyReceiptUpdate core below.
// ---------------------------------------------------------------------------

func (m *Manager) handleEvent(accountID string, raw any) {
	ctx := context.Background()
	switch evt := raw.(type) {
	case *events.Message:
		m.handleMessageEvent(ctx, accountID, evt)
	case *events.Receipt:
		if neutral, ok := translateReceipt(accountID, evt); ok {
			m.applyReceiptUpdate(ctx, neutral)
		}
	case *events.Connected:
		m.setConnectionState(ctx, accountID, "connected", "")
	case *events.Disconnected:
		m.setConnectionState(ctx, accountID, "disconnected", "")
	case *events.LoggedOut:
		m.handleLoggedOut(ctx, accountID, evt)
	case *events.StreamReplaced:
		m.setConnectionState(ctx, accountID, "disconnected", "stream replaced by another connection")
	case *events.ClientOutdated:
		m.setConnectionState(ctx, accountID, "disconnected", "client outdated")
	case *events.TemporaryBan:
		m.setConnectionState(ctx, accountID, "disconnected", evt.String())
	case *events.ConnectFailure:
		m.setConnectionState(ctx, accountID, "disconnected", evt.Message)
	case *events.HistorySync, *events.OfflineSyncPreview, *events.OfflineSyncCompleted, *events.KeepAliveTimeout:
		m.log.Debug("whatsmeow event dropped", "account_id", accountID, "event", fmt.Sprintf("%T", raw))
	case *events.PairSuccess, *events.PairError:
		// Handled by Pair's own, pairing-scoped handler on the same client
		// (see pairing.go) — nothing to do here.
	default:
		// Presence, app-state sync, and everything else this inbox does not
		// act on are intentionally ignored.
	}
}

func (m *Manager) handleMessageEvent(ctx context.Context, accountID string, evt *events.Message) {
	if evt.Info.IsGroup {
		m.log.Info("whatsmeow event dropped", "account_id", accountID, "event", "message", "result", "group")
		return
	}
	neutral := translateMessage(accountID, evt)
	res, err := m.ingestMessage(ctx, neutral)
	if err != nil {
		return
	}
	if res.MessageInserted && neutral.Media != nil {
		m.downloadAndAttachMedia(accountID, res.MessageID, evt.Message)
	}
}

// ingestMessage is the shared core for a real inbound/echoed message and a
// synthetic /debug/wa-event "message": store upsert, SSE broadcast, and (for
// a genuinely new inbound message) the AI-draft enqueue.
func (m *Manager) ingestMessage(ctx context.Context, evt whatsapp.MessageReceived) (store.InboundResult, error) {
	accountID, err := uuid.Parse(evt.AccountID)
	if err != nil {
		return store.InboundResult{}, fmt.Errorf("whatsmeow: invalid account id %q: %w", evt.AccountID, err)
	}

	direction, senderKind := "in", "contact"
	if evt.FromMe {
		direction, senderKind = "out", "external_account"
	}
	res, err := m.cfg.Store.UpsertInbound(ctx, store.InboundUpsert{
		AccountID:         accountID,
		PhoneJID:          evt.PhoneJID,
		LidJID:            evt.LidJID,
		RemoteJID:         evt.ChatJID,
		PhoneNumber:       config.PhoneFromJID(evt.PhoneJID),
		PushName:          evt.PushName,
		Direction:         direction,
		SenderKind:        senderKind,
		ExternalMessageID: evt.ExternalID,
		MessageKind:       evt.Kind,
		Body:              evt.Text,
		Preview:           previewFor(evt),
		Source:            "live_webhook",
		Raw:               evt.Raw,
		MessageTS:         evt.Timestamp,
	})
	if err != nil {
		m.log.Error("whatsmeow: ingest failed", "account_id", accountID, "err", err)
		return store.InboundResult{}, err
	}

	if !res.MessageInserted {
		// Dedup, or the fromMe echo collapsing onto the send that produced
		// it — enrichment only, matching the pre-whatsmeow worker's own
		// "duplicate" branch.
		m.broadcastMessage(ctx, res.MessageID, "message.updated")
		return res, nil
	}

	m.broadcastMessage(ctx, res.MessageID, "message.created")
	if res.ChatCreated {
		m.broadcastChat(ctx, res.ChatID, "chat.created")
	} else {
		m.broadcastChat(ctx, res.ChatID, "chat.updated")
	}
	if evt.Media != nil {
		ref := store.MediaRef{MediaType: evt.Media.Kind, Mimetype: evt.Media.Mimetype, FileName: evt.Media.FileName, FileSize: int(evt.Media.FileSize)}
		if _, _, err := m.cfg.Store.UpsertMessageMedia(ctx, res.MessageID, ref, mediaBlobID(res.MessageID), "pending"); err != nil {
			m.log.Error("whatsmeow: upsert media row failed", "message_id", res.MessageID, "err", err)
		} else {
			m.broadcastMessage(ctx, res.MessageID, "message.updated")
		}
	}
	if direction == "in" {
		m.enqueueAIDraft(ctx, res.ChatID)
	}
	return res, nil
}

func mediaBlobID(messageID uuid.UUID) string { return "wa-" + messageID.String() }

// downloadAndAttachMedia fetches an inbound attachment's bytes in the
// background (off the account's own event loop, so a large file never
// delays the next message/receipt) and marks the media row ready.
func (m *Manager) downloadAndAttachMedia(accountID string, messageID uuid.UUID, msg *waE2E.Message) {
	mc, ok := m.clientFor(accountID)
	if !ok {
		m.log.Warn("whatsmeow: media download skipped: account not connected", "account_id", accountID, "message_id", messageID)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mediaDownloadTimeout)
		defer cancel()
		data, err := downloadMedia(ctx, mc.client, msg)
		if err != nil {
			m.log.Error("whatsmeow: media download failed", "message_id", messageID, "err", err)
			return
		}
		_, _, media := extractContent(msg)
		meta := blob.Meta{FileSize: int64(len(data))}
		if media != nil {
			meta.MediaType, meta.Mimetype, meta.FileName = media.Kind, media.Mimetype, media.FileName
		}
		if _, err := m.cfg.Blob.Put(mediaBlobID(messageID), data, meta); err != nil {
			m.log.Error("whatsmeow: store media blob failed", "message_id", messageID, "err", err)
			return
		}
		if err := m.cfg.Store.SetMediaReady(ctx, messageID, len(data)); err != nil {
			m.log.Error("whatsmeow: mark media ready failed", "message_id", messageID, "err", err)
			return
		}
		m.broadcastMessage(ctx, messageID, "message.updated")
	}()
}

// previewFor mirrors the pre-whatsmeow worker's own chat-list preview rule:
// truncated text, or a media placeholder when there is none.
func previewFor(evt whatsapp.MessageReceived) string {
	if evt.Text != "" {
		r := []rune(evt.Text)
		if len(r) > 120 {
			return string(r[:120])
		}
		return evt.Text
	}
	if evt.Media != nil {
		switch evt.Media.Kind {
		case "image":
			return "📷 Фото"
		case "video":
			return "🎥 Видео"
		case "audio":
			return "🎙 Аудио"
		case "document":
			return "📄 Документ"
		case "sticker":
			return "🌟 Стикер"
		}
	}
	return ""
}

// deliveryRank mirrors internal/store.AdvanceDeliveryState's own ladder —
// duplicated here (rather than exported from internal/store) because it is
// the caller's job to know which rank it is asking to advance to.
func deliveryRank(state string) int {
	switch state {
	case "queued":
		return 0
	case "sent":
		return 1
	case "delivered":
		return 2
	case "read":
		return 3
	case "failed":
		return 4
	default:
		return 0
	}
}

// applyReceiptUpdate is the shared core for a real receipt and a synthetic
// /debug/wa-event "receipt": both advance the same monotonic delivery ladder
// for every external id the receipt names. An id that does not exist, or is
// not a forward transition, is silently skipped — never an error.
func (m *Manager) applyReceiptUpdate(ctx context.Context, evt whatsapp.ReceiptUpdated) {
	accountID, err := uuid.Parse(evt.AccountID)
	if err != nil {
		m.log.Error("whatsmeow: invalid account id on receipt", "account_id", evt.AccountID, "err", err)
		return
	}
	rank := deliveryRank(evt.DeliveryState)
	for _, extID := range evt.ExternalIDs {
		msgID, _, err := m.cfg.Store.AdvanceDeliveryState(ctx, accountID, extID, evt.DeliveryState, rank)
		if err != nil {
			if err != store.ErrNotFound {
				m.log.Error("whatsmeow: advance delivery state failed", "account_id", accountID, "external_id", extID, "err", err)
			}
			continue
		}
		m.broadcastMessage(ctx, msgID, "message.updated")
	}
}

func (m *Manager) setConnectionState(ctx context.Context, accountID, state, reason string) {
	id, err := uuid.Parse(accountID)
	if err != nil {
		m.log.Error("whatsmeow: invalid account id", "account_id", accountID, "err", err)
		return
	}
	if err := m.cfg.Store.SetAccountState(ctx, id, state); err != nil {
		m.log.Error("whatsmeow: set account state failed", "account_id", accountID, "state", state, "err", err)
		return
	}
	if reason != "" {
		m.log.Warn("whatsmeow: connection state changed", "account_id", accountID, "state", state, "reason", reason)
	} else {
		m.log.Info("whatsmeow: connection state changed", "account_id", accountID, "state", state)
	}
	if acct, err := m.cfg.Store.AccountByID(ctx, id); err == nil {
		m.cfg.Hub.Broadcast("wa_account.status_changed", dto.MapAccount(acct, state))
	}
}

// handleLoggedOut reacts to an EXTERNAL logout (the server or the phone
// ending the session) — as opposed to Manager.Logout, which is this
// process's own user-initiated request and does its own cleanup directly.
func (m *Manager) handleLoggedOut(ctx context.Context, accountID string, evt *events.LoggedOut) {
	id, err := uuid.Parse(accountID)
	if err != nil {
		m.log.Error("whatsmeow: invalid account id on logout", "account_id", accountID, "err", err)
		return
	}
	if err := m.cfg.Store.DeleteWaCredentials(ctx, id); err != nil {
		m.log.Error("whatsmeow: delete credentials on logout failed", "account_id", accountID, "err", err)
	}
	// The session is dead server-side, so drop our end too rather than
	// leaving the consumer goroutine and websocket alive for an account that
	// can never receive anything again. stop() is safe to call from this
	// goroutine (it IS the consumer, and the loop exits on the next turn)
	// and safe to race with Logout/Close — see managedClient.stop.
	if mc, ok := m.clientFor(accountID); ok {
		m.removeClient(accountID)
		mc.stop()
		mc.client.Disconnect()
	}
	m.setConnectionState(ctx, accountID, "logged_out",
		fmt.Sprintf("external logout (on_connect=%v reason=%v)", evt.OnConnect, evt.Reason))
}

func (m *Manager) enqueueAIDraft(ctx context.Context, chatID uuid.UUID) {
	pctx, cancel := context.WithTimeout(ctx, aiDraftEnqueueTimeout)
	defer cancel()
	if err := m.cfg.Queue.Publish(pctx, queue.Message{Kind: queue.KindAIDraft, Payload: worker.AIDraftTask{ChatID: chatID}}); err != nil {
		m.log.Error("whatsmeow: enqueue ai draft failed", "chat_id", chatID, "err", err)
	}
}

func (m *Manager) broadcastMessage(ctx context.Context, id uuid.UUID, name string) {
	msg, err := m.cfg.Store.MessageByID(ctx, id)
	if err != nil {
		m.log.Error("whatsmeow: load message for broadcast failed", "message_id", id, "err", err)
		return
	}
	m.cfg.Hub.Broadcast(name, dto.MapMessage(msg))
}

func (m *Manager) broadcastChat(ctx context.Context, id uuid.UUID, name string) {
	chat, err := m.cfg.Store.ChatByID(ctx, id)
	if err != nil {
		m.log.Error("whatsmeow: load chat for broadcast failed", "chat_id", id, "err", err)
		return
	}
	m.cfg.Hub.Broadcast(name, dto.MapChat(chat))
}

// ---------------------------------------------------------------------------
// whatsapp.Manager port
// ---------------------------------------------------------------------------

// Status returns an account's current connection state, read from the store
// — the single source of truth every event handler above also writes to.
func (m *Manager) Status(ctx context.Context, accountID string) (whatsapp.ConnectionChanged, error) {
	id, err := uuid.Parse(accountID)
	if err != nil {
		return whatsapp.ConnectionChanged{}, fmt.Errorf("whatsmeow: invalid account id %q: %w", accountID, err)
	}
	acct, err := m.cfg.Store.AccountByID(ctx, id)
	if err != nil {
		return whatsapp.ConnectionChanged{}, err
	}
	return whatsapp.ConnectionChanged{AccountID: accountID, State: acct.ConnectionState}, nil
}

// Logout is this process's own user-initiated disconnect: it stops and
// disconnects the live client (if any), deletes the whatsmeow device
// session (Client.Logout does both server-side unlink and local deletion),
// then removes our credential mapping and marks the account logged out —
// best-effort past the first failure, so a network hiccup mid-logout never
// leaves the account looking permanently "connected".
func (m *Manager) Logout(ctx context.Context, accountID string) error {
	id, err := uuid.Parse(accountID)
	if err != nil {
		return fmt.Errorf("whatsmeow: invalid account id %q: %w", accountID, err)
	}
	if mc, ok := m.clientFor(accountID); ok {
		m.removeClient(accountID)
		mc.stop()
		if err := mc.client.Logout(ctx); err != nil {
			m.log.Warn("whatsmeow: logout call failed; continuing local cleanup", "account_id", accountID, "err", err)
		}
	}
	if err := m.cfg.Store.DeleteWaCredentials(ctx, id); err != nil {
		return fmt.Errorf("whatsmeow: delete credentials: %w", err)
	}
	return m.cfg.Store.SetAccountState(ctx, id, "logged_out")
}

// ChannelSender returns the messaging.ChannelSender that routes approved
// outbound sends through this manager's live per-account clients.
func (m *Manager) ChannelSender() messaging.ChannelSender {
	return &Sender{manager: m}
}

// InjectDebugEvent feeds a synthetic event through the exact same
// ingestMessage/applyReceiptUpdate/setConnectionState core a real whatsmeow
// event takes — see whatsapp.DebugEvent's doc comment.
func (m *Manager) InjectDebugEvent(ctx context.Context, evt whatsapp.DebugEvent) (whatsapp.DebugEventResult, error) {
	if evt.AccountID == "" {
		return whatsapp.DebugEventResult{}, fmt.Errorf("whatsmeow: account_id is required")
	}
	switch evt.EventType {
	case "message":
		return m.injectDebugMessage(ctx, evt)
	case "receipt":
		return whatsapp.DebugEventResult{}, m.injectDebugReceipt(ctx, evt)
	case "connected", "disconnected", "logged_out":
		m.setConnectionState(ctx, evt.AccountID, evt.EventType, "debug event")
		return whatsapp.DebugEventResult{}, nil
	default:
		return whatsapp.DebugEventResult{}, fmt.Errorf("whatsmeow: unknown event_type %q", evt.EventType)
	}
}

func (m *Manager) injectDebugMessage(ctx context.Context, evt whatsapp.DebugEvent) (whatsapp.DebugEventResult, error) {
	phoneJID, lidJID := debugContactRefs(evt.SenderJID)
	externalID := evt.ExternalID
	if externalID == "" {
		externalID = uuid.NewString()
	}
	neutral := whatsapp.MessageReceived{
		AccountID: evt.AccountID, ExternalID: externalID,
		ChatJID: whatsapp.ChatKey(phoneJID, lidJID), PhoneJID: phoneJID, LidJID: lidJID,
		PushName: evt.SenderJID, FromMe: evt.IsFromMe, Kind: "conversation", Text: evt.Text,
		Timestamp: time.Now(),
	}
	res, err := m.ingestMessage(ctx, neutral)
	if err != nil {
		return whatsapp.DebugEventResult{}, err
	}
	return whatsapp.DebugEventResult{MessageID: res.MessageID.String(), ChatID: res.ChatID.String()}, nil
}

func (m *Manager) injectDebugReceipt(ctx context.Context, evt whatsapp.DebugEvent) error {
	state, ok := debugReceiptState(evt.ReceiptType)
	if !ok {
		return fmt.Errorf("whatsmeow: unknown receipt_type %q", evt.ReceiptType)
	}
	if evt.ExternalID == "" {
		return fmt.Errorf("whatsmeow: external_id is required for a receipt event")
	}
	m.applyReceiptUpdate(ctx, whatsapp.ReceiptUpdated{
		AccountID: evt.AccountID, ExternalIDs: []string{evt.ExternalID}, DeliveryState: state,
	})
	return nil
}

// debugReceiptState mirrors deliveryStateForReceipt's mapping, over the
// debug endpoint's own plain-string receipt_type field.
func debugReceiptState(receiptType string) (string, bool) {
	switch receiptType {
	case "":
		return "delivered", true
	case "read", "played":
		return "read", true
	default:
		return "", false
	}
}

// debugContactRefs runs a debug request's raw sender_jid (a bare phone
// number, a full phone JID, or an "@lid" address) through the SAME
// canonicalization real inbound events use, so the debug endpoint genuinely
// exercises JID normalization rather than bypassing it.
func debugContactRefs(raw string) (phoneJID, lidJID string) {
	canon := config.CanonicalJID(raw)
	if whatsapp.IsLID(canon) {
		return "", canon
	}
	return canon, ""
}
