package whatsmeow

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"

	wm "go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
)

// qrPNGSize is the rendered QR code's side length in pixels — comfortably
// scannable on a phone camera without producing an oversized data URI.
const qrPNGSize = 256

// pairingResultRetention is how long a terminal pairing session (connected,
// timeout, or error) stays in the registry after its last update — long
// enough for one final frontend poll to observe it, short enough that
// abandoned pairing attempts do not accumulate forever.
const pairingResultRetention = 2 * time.Minute

// pairingRegistry tracks in-flight (and recently finished) QR pairing
// sessions by id, so PairStatus's poller and the background pairing
// goroutines can hand off state without blocking each other.
type pairingRegistry struct {
	mu       sync.Mutex
	sessions map[string]whatsapp.PairingUpdate
}

func newPairingRegistry() *pairingRegistry {
	return &pairingRegistry{sessions: make(map[string]whatsapp.PairingUpdate)}
}

func (r *pairingRegistry) set(update whatsapp.PairingUpdate) {
	r.mu.Lock()
	r.sessions[update.SessionID] = update
	r.mu.Unlock()
}

// finish records a terminal update and schedules its own removal — the
// registry must not grow forever across many pairing attempts over a long
// process lifetime.
func (r *pairingRegistry) finish(update whatsapp.PairingUpdate) {
	r.set(update)
	time.AfterFunc(pairingResultRetention, func() {
		r.mu.Lock()
		if cur, ok := r.sessions[update.SessionID]; ok && cur.Status == update.Status {
			delete(r.sessions, update.SessionID)
		}
		r.mu.Unlock()
	})
}

func (r *pairingRegistry) get(sessionID string) (whatsapp.PairingUpdate, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.sessions[sessionID]
	return u, ok
}

// Pair starts a new QR pairing flow for orgID: a fresh whatsmeow device, a
// QR channel consumed in the background (each code re-rendered as a PNG and
// published to the session), ending in either a paired account or a
// timeout/error. See whatsapp.Manager.Pair's doc comment for the contract.
func (m *Manager) Pair(ctx context.Context, orgID string) (string, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return "", fmt.Errorf("whatsmeow: invalid organization id %q: %w", orgID, err)
	}

	device := m.container.NewDevice()
	cli := m.newClient(device, NewLogger(m.log, "whatsmeow-pairing"))

	// Per whatsmeow's own contract, GetQRChannel must be called before Connect.
	qrChan, err := cli.GetQRChannel(context.Background())
	if err != nil {
		return "", fmt.Errorf("whatsmeow: get qr channel: %w", err)
	}

	sessionID := uuid.NewString()
	m.pairings.set(whatsapp.PairingUpdate{SessionID: sessionID, Status: "qr_required"})

	// A pairing-scoped handler alongside (not instead of) the normal
	// per-account one registerClient adds once pairing succeeds — whatsmeow
	// dispatches every event to every registered handler, so both coexist
	// harmlessly; this one only ever reacts to PairSuccess/PairError, which
	// cannot recur once the client is fully registered.
	cli.AddEventHandler(func(evt any) {
		switch e := evt.(type) {
		case *events.PairSuccess:
			m.completePairing(cli, oid, sessionID, e.ID)
		case *events.PairError:
			m.pairings.finish(whatsapp.PairingUpdate{SessionID: sessionID, Status: "error", Message: e.Error.Error()})
		}
	})

	go m.consumeQR(sessionID, qrChan)

	if err := cli.Connect(); err != nil {
		return "", fmt.Errorf("whatsmeow: connect: %w", err)
	}
	return sessionID, nil
}

// completePairing runs once, on PairSuccess: it derives the permanent
// account id from the paired phone number, persists the account and the
// device-JID mapping, and hands cli over to the normal per-account event
// dispatcher — from this point on the account is indistinguishable from one
// reconnected at boot.
func (m *Manager) completePairing(cli waClient, orgID uuid.UUID, sessionID string, deviceJID types.JID) {
	ctx := context.Background()
	ownerJID := resolveOwnerJID(deviceJID)

	acct, err := m.cfg.Store.UpsertConnectedAccount(ctx, store.Account{
		ID: config.AccountID(ownerJID),
		// DisplayName is left blank on a fresh pair: dto.MapAccount/
		// MapNeutralAccount fall back to the phone number, which is more
		// useful across multiple accounts than a repeated generic label.
		OrganizationID:     uuid.NullUUID{UUID: orgID, Valid: true},
		ExternalAccountRef: ownerJID,
		ExternalHandle:     config.PhoneFromJID(ownerJID),
		ConnectionState:    "connected",
	})
	if err != nil {
		m.log.Error("whatsmeow: finalize pairing failed", "session_id", sessionID, "err", err)
		m.pairings.finish(whatsapp.PairingUpdate{SessionID: sessionID, Status: "error", Message: err.Error()})
		return
	}
	// The RAW (AD-form) device JID is what container.GetDevice needs to look
	// the session back up on reconnect — never the canonicalized owner JID.
	if err := m.cfg.Store.SaveWaCredentials(ctx, acct.ID, deviceJID.String()); err != nil {
		m.log.Error("whatsmeow: save credentials failed", "account_id", acct.ID, "err", err)
	}

	m.registerClient(acct.ID.String(), cli)

	m.log.Info("whatsmeow: account paired", "account_id", acct.ID, "owner_jid", ownerJID)
	m.cfg.Hub.Broadcast("wa_account.status_changed", dto.MapAccount(acct, "connected"))
	m.pairings.finish(whatsapp.PairingUpdate{SessionID: sessionID, Status: "connected", AccountID: acct.ID.String()})
}

// consumeQR renders each QR code whatsmeow issues as a PNG and publishes it
// to the session; it returns on its own once whatsmeow closes the channel
// (success, timeout, or a terminal pairing error).
func (m *Manager) consumeQR(sessionID string, qrChan <-chan wm.QRChannelItem) {
	for item := range qrChan {
		switch item.Event {
		case wm.QRChannelEventCode:
			update := whatsapp.PairingUpdate{SessionID: sessionID, Status: "qr_required", QRCode: item.Code}
			png, err := qrcode.Encode(item.Code, qrcode.Medium, qrPNGSize)
			if err != nil {
				m.log.Error("whatsmeow: render qr png failed", "session_id", sessionID, "err", err)
			} else {
				update.QRBase64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
			}
			m.pairings.set(update)
		case wm.QRChannelSuccess.Event:
			// completePairing (via the PairSuccess event handler) already
			// recorded the "connected" status.
		case wm.QRChannelTimeout.Event:
			m.pairings.finish(whatsapp.PairingUpdate{SessionID: sessionID, Status: "timeout"})
		default:
			msg := item.Event
			if item.Error != nil {
				msg = item.Error.Error()
			}
			m.pairings.finish(whatsapp.PairingUpdate{SessionID: sessionID, Status: "error", Message: msg})
		}
	}
}

// PairStatus returns the latest known state of a pairing session.
func (m *Manager) PairStatus(sessionID string) (whatsapp.PairingUpdate, bool) {
	return m.pairings.get(sessionID)
}
