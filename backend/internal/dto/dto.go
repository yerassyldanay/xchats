// Package dto holds the JSON entity shapes shared by the HTTP API (responses) and
// the realtime layer (SSE), with mappers from the store types. One shape, two
// emitters — so a Chat over SSE is byte-identical to a Chat from GET /chats.
package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/automation"
	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

func decodeAttrs(b []byte) map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// Contact is the API contact shape.
type Contact struct {
	ID          string         `json:"id"`
	DisplayName string         `json:"display_name"`
	PhoneNumber string         `json:"phone_number"`
	PhoneJID    string         `json:"phone_jid"`
	LidJID      string         `json:"lid_jid"`
	PushName    string         `json:"push_name"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

// Chat is the API chat shape.
type Chat struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	AccountID string `json:"account_id"`
	// WaAccountID is the deprecated alias for AccountID, emitted only for chats
	// on the wa_* gateway (whatsapp/simulator) so an older client keeps working
	// through the transition. Every other channel deliberately omits it rather
	// than pretending its account is a WhatsApp one.
	WaAccountID        string  `json:"wa_account_id,omitempty"`
	Contact            Contact `json:"contact"`
	Status             string  `json:"status"`
	AssigneeUserID     *string `json:"assignee_user_id"`
	UnreadCount        int     `json:"unread_count"`
	LastMessageAt      *string `json:"last_message_at"`
	LastMessagePreview string  `json:"last_message_preview"`
	// CustomerID is the CRM customer this conversation belongs to — what the
	// customer sidebar hydrates from without a second chat-scoped route. Null
	// for a chat on an unassigned account and for chats that predate the CRM
	// migration.
	CustomerID *string `json:"customer_id"`
}

// Media is one media item on a message (a "list of URLs", each enriched).
type Media struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
	Mimetype  string `json:"mimetype"`
	FileName  string `json:"file_name"`
	FileSize  int    `json:"file_size"`
}

// Message is the API message shape.
type Message struct {
	ID                string  `json:"id"`
	ChatID            string  `json:"chat_id"`
	Direction         string  `json:"direction"`
	SenderType        string  `json:"sender_type"`
	SenderUserID      *string `json:"sender_user_id"`
	ExternalMessageID string  `json:"external_message_id"`
	MessageType       string  `json:"message_type"`
	Content           string  `json:"content"`
	Media             []Media `json:"media"`
	Status            string  `json:"status"`
	Source            string  `json:"source"`
	Timestamp         *string `json:"timestamp"`
}

// AiDraft is one suggested option.
type AiDraft struct {
	ID               string   `json:"id"`
	ChatID           string   `json:"chat_id"`
	TriggerMessageID *string  `json:"trigger_message_id"`
	Ordinal          int      `json:"ordinal"`
	DraftText        string   `json:"draft_text"`
	ReplyLanguage    string   `json:"reply_language"`
	ContextStatus    string   `json:"context_status"`
	Confidence       *float64 `json:"confidence"`
	Escalate         bool     `json:"escalate"`
	EscalationReason string   `json:"escalation_reason"`
	Status           string   `json:"status"`
	CreatedAt        string   `json:"created_at"`
}

// WhatsAppAccount is the API shape for one connected number. "assigned" is
// derived (organization_id IS NOT NULL), never stored.
type WhatsAppAccount struct {
	ID               string  `json:"id"`
	DisplayName      string  `json:"display_name"`
	ConnectionStatus string  `json:"connection_status"`
	Assigned         bool    `json:"assigned"`
	OwnerJID         string  `json:"owner_jid"`
	PhoneNumber      string  `json:"phone_number"`
	LastLiveEventAt  *string `json:"last_live_event_at"`
	CreatedAt        *string `json:"created_at"`
}

// MapAccount maps a store.Account. liveStatus overrides the stored connection_state
// when a live probe is available (else pass the stored state). A blank
// display_name falls back to the phone number for the UI.
func MapAccount(a store.Account, liveStatus string) WhatsAppAccount {
	name := a.DisplayName
	if name == "" {
		name = a.ExternalHandle
	}
	status := liveStatus
	if status == "" {
		status = a.ConnectionState
	}
	return WhatsAppAccount{
		ID:               a.ID.String(),
		DisplayName:      name,
		ConnectionStatus: status,
		Assigned:         a.OrganizationID.Valid,
		OwnerJID:         a.ExternalAccountRef,
		PhoneNumber:      a.ExternalHandle,
		LastLiveEventAt:  tsPtr(a.LastLiveEventAt),
		CreatedAt:        tsPtr(&a.CreatedAt),
	}
}

// Account is the channel-neutral account shape GET /accounts returns — the one
// list the UI renders for every channel. The webhook_* fields are Telegram
// health and stay null on a WhatsApp row.
type Account struct {
	ID              string  `json:"id"`
	Channel         string  `json:"channel"`
	DisplayName     string  `json:"display_name"`
	ExternalHandle  string  `json:"external_handle"`
	ConnectionState string  `json:"connection_state"`
	Assigned        bool    `json:"assigned"`
	LastLiveEventAt *string `json:"last_live_event_at"`
	CreatedAt       *string `json:"created_at"`

	WebhookURL           *string `json:"webhook_url"`
	WebhookRegisteredAt  *string `json:"webhook_registered_at"`
	WebhookLastCheckedAt *string `json:"webhook_last_checked_at"`
	WebhookLastError     *string `json:"webhook_last_error"`

	// Automation is this channel's effective (resolved) debounce/scheduled
	// auto-reply configuration — see MapAccountAutomation.
	Automation AccountAutomation `json:"automation"`
}

// ScheduleWindow is one recurring UTC weekday/time-of-day range — the wire
// shape of store.AutomationWindow (minus its id/account, which the API never
// exposes: a save always replaces the whole set, so there is nothing to
// address an individual window by).
type ScheduleWindow struct {
	// Weekday is 0=Sunday..6=Saturday in UTC (time.Weekday's own numbering).
	Weekday     int `json:"weekday"`
	StartMinute int `json:"start_minute"`
	EndMinute   int `json:"end_minute"`
}

// AccountAutomation is one channel's automation settings, wire-ready:
// WaitSeconds is always the EFFECTIVE (resolved) wait so the UI never has to
// duplicate the override-vs-default resolution logic itself;
// WaitSecondsOverride is the raw stored value (null means "using the
// system default") so the dialog can still distinguish "explicitly set to
// the same number as the default" from "not customized".
type AccountAutomation struct {
	Mode                string           `json:"mode"`
	WaitSeconds         int              `json:"wait_seconds"`
	WaitSecondsOverride *int             `json:"wait_seconds_override"`
	DefaultWaitSeconds  int              `json:"default_wait_seconds"`
	Schedule            []ScheduleWindow `json:"schedule"`
}

// MapAccountAutomation resolves store.AutomationSettings/AutomationWindow
// into the wire shape. settings zero-valued (Mode=="") is treated as the
// implicit default automation.DefaultMode — store.AutomationSettingsForAccount
// already returns that default for an unconfigured account, but callers
// bulk-loading via AutomationSettingsForAccounts get a zero value for an
// absent map entry instead, so resolving the default here too means every
// caller gets the same answer regardless of which path it took.
func MapAccountAutomation(settings store.AutomationSettings, windows []store.AutomationWindow, systemDefaultWait int) AccountAutomation {
	mode := settings.Mode
	if mode == "" {
		mode = string(automation.DefaultMode)
	}
	sched := make([]ScheduleWindow, 0, len(windows))
	for _, w := range windows {
		sched = append(sched, ScheduleWindow{Weekday: w.Weekday, StartMinute: w.StartMinute, EndMinute: w.EndMinute})
	}
	return AccountAutomation{
		Mode:                mode,
		WaitSeconds:         automation.EffectiveWaitSeconds(systemDefaultWait, settings.WaitSecondsOverride),
		WaitSecondsOverride: settings.WaitSecondsOverride,
		DefaultWaitSeconds:  systemDefaultWait,
		Schedule:            sched,
	}
}

// MapNeutralAccount maps a store.Account (as read from inbox_accounts_v) to the
// neutral API shape. liveStatus overrides the stored state when a live probe is
// available, exactly like MapAccount.
func MapNeutralAccount(a store.Account, liveStatus string) Account {
	name := a.DisplayName
	if name == "" {
		name = a.ExternalHandle
	}
	status := liveStatus
	if status == "" {
		status = a.ConnectionState
	}
	out := Account{
		ID:              a.ID.String(),
		Channel:         a.Channel,
		DisplayName:     name,
		ExternalHandle:  a.ExternalHandle,
		ConnectionState: status,
		Assigned:        a.OrganizationID.Valid,
		LastLiveEventAt: tsPtr(a.LastLiveEventAt),
		CreatedAt:       tsPtr(&a.CreatedAt),
	}
	// Only a channel that actually has webhook health reports it; a WhatsApp row
	// leaves all four null rather than emitting empty strings that read as "set".
	if hasWebhookHealth(a.Channel) {
		url, lastErr := a.WebhookURL, a.WebhookLastError
		out.WebhookURL = &url
		out.WebhookLastError = &lastErr
		out.WebhookRegisteredAt = tsPtr(a.WebhookRegisteredAt)
		out.WebhookLastCheckedAt = tsPtr(a.WebhookLastCheckedAt)
	}
	return out
}

// hasWebhookHealth reports whether a channel registers a provider webhook
// and so has health fields worth reporting. WhatsApp (whatsmeow, a direct
// WebSocket connection) and the simulator have no webhook at all; Telegram
// and every Meta channel (Instagram, Messenger, WhatsApp Cloud API) do.
func hasWebhookHealth(channel string) bool {
	switch messaging.Channel(channel) {
	case messaging.ChannelTelegram, messaging.ChannelInstagram, messaging.ChannelMessenger, messaging.ChannelWhatsAppCloud:
		return true
	default:
		return false
	}
}

func mediaURL(id uuid.UUID) string { return "/xchats/api/v1/media/" + id.String() }

func tsPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func nullUUIDPtr(u uuid.NullUUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.UUID.String()
	return &s
}

// MapContact maps a store.Contact.
func MapContact(c store.Contact) Contact {
	name := c.DisplayName
	if name == "" {
		name = c.PushName
	}
	if name == "" {
		name = c.PhoneNumber
	}
	return Contact{
		ID:          c.ID.String(),
		DisplayName: name,
		PhoneNumber: c.PhoneNumber,
		PhoneJID:    c.ExternalContactRef,
		LidJID:      c.LidJID,
		PushName:    c.PushName,
		Attributes:  decodeAttrs(c.Attributes),
	}
}

// MapChat maps a store.Chat (with embedded contact).
func MapChat(c store.Chat) Chat {
	channel := c.Channel
	if channel == "" {
		channel = "whatsapp"
	}
	out := Chat{
		ID:                 c.ID.String(),
		Channel:            channel,
		AccountID:          c.AccountID.String(),
		Contact:            MapContact(c.Contact),
		Status:             c.ChatState,
		AssigneeUserID:     nullUUIDPtr(c.AssigneeUserID),
		UnreadCount:        c.UnreadCount,
		LastMessageAt:      tsPtr(c.LastMessageAt),
		LastMessagePreview: c.LastMessagePreview,
		CustomerID:         nullUUIDPtr(c.CustomerID),
	}
	if channel == string(messaging.ChannelWhatsApp) || channel == string(messaging.ChannelSimulator) {
		out.WaAccountID = c.AccountID.String()
	}
	return out
}

// MapMessage maps a store.Message (with media).
func MapMessage(m store.Message) Message {
	media := make([]Media, 0, len(m.Media))
	for _, r := range m.Media {
		media = append(media, Media{
			ID: r.ID.String(), URL: mediaURL(r.ID), MediaType: r.MediaType,
			Mimetype: r.Mimetype, FileName: r.FileName, FileSize: r.FileSize,
		})
	}
	return Message{
		ID:                m.ID.String(),
		ChatID:            m.ChatID.String(),
		Direction:         m.Direction,
		SenderType:        m.SenderKind,
		SenderUserID:      nullUUIDPtr(m.SenderUserID),
		ExternalMessageID: m.ExternalMessageID,
		MessageType:       m.MessageKind,
		Content:           m.Body,
		Media:             media,
		Status:            m.DeliveryState,
		Source:            m.Source,
		Timestamp:         tsPtr(m.MessageTS),
	}
}

// MapDraft maps a store.Draft.
func MapDraft(d store.Draft) AiDraft {
	return AiDraft{
		ID:               d.ID.String(),
		ChatID:           d.ChatID.String(),
		TriggerMessageID: nullUUIDPtr(d.TriggerMessageID),
		Ordinal:          d.OptionOrdinal,
		DraftText:        d.DraftText,
		ReplyLanguage:    d.ReplyLanguage,
		ContextStatus:    d.ContextState,
		Confidence:       d.Confidence,
		Escalate:         d.Escalate,
		EscalationReason: d.EscalationReason,
		Status:           d.DraftState,
		CreatedAt:        d.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// ---------------------------------------------------------------------------
// Campaigns
// ---------------------------------------------------------------------------
//
// CampaignStatusEvent/CampaignRecipientEvent are also the exact SSE payload
// shapes internal/campaign's Scheduler/Runner broadcast (imported from
// there, not redeclared) — ids and an enum status string only, NEVER a
// recipient's raw_input/normalized_identity/name. internal/realtime.Hub is
// process-global, not scoped to an organization: every connected client on
// every org receives every event, so a recipient's phone number (or
// Telegram chat id) must never ride one of these. See plan/DECISIONS.md.

// CampaignWindow is one recurring UTC weekday/time-of-day range — the wire
// shape of store.CampaignWindow (minus its id, exactly like automation's own
// ScheduleWindow: a save always replaces the whole set, so there is nothing
// to address an individual window by).
type CampaignWindow struct {
	// Weekday is 0=Sunday..6=Saturday in UTC.
	Weekday     int `json:"weekday"`
	StartMinute int `json:"start_minute"`
	EndMinute   int `json:"end_minute"`
}

// MapCampaignWindows converts store.CampaignWindow rows to their wire shape.
func MapCampaignWindows(ws []store.CampaignWindow) []CampaignWindow {
	out := make([]CampaignWindow, 0, len(ws))
	for _, w := range ws {
		out = append(out, CampaignWindow{Weekday: w.Weekday, StartMinute: w.StartMinute, EndMinute: w.EndMinute})
	}
	return out
}

// Campaign is the API campaign shape.
type Campaign struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	AccountID          string           `json:"account_id"`
	Channel            string           `json:"channel"`
	Status             string           `json:"status"`
	MessageBody        string           `json:"message_body"`
	Variables          []string         `json:"variables"`
	MinIntervalSeconds *int             `json:"min_interval_seconds"`
	JitterSeconds      *int             `json:"jitter_seconds"`
	Windows            []CampaignWindow `json:"windows"`
	ScheduleAt         *string          `json:"schedule_at"`
	StartedAt          *string          `json:"started_at"`
	CreatedBy          string           `json:"created_by"`
	CreatedAt          string           `json:"created_at"`
	UpdatedAt          string           `json:"updated_at"`
	// RecipientCounts keys by recipient status (pending/sending/sent/failed/
	// skipped) — the list/detail views' progress bars.
	RecipientCounts map[string]int `json:"recipient_counts"`
}

// MapCampaign maps a store.Campaign plus its own recurring windows and live
// recipient-status counts, which the store keeps as three separate reads
// (campaigns is the hot row a status transition rewrites every send;
// windows and counts are read-mostly and would only add churn to that row).
func MapCampaign(c store.Campaign, windows []store.CampaignWindow, counts map[string]int) Campaign {
	if counts == nil {
		counts = map[string]int{}
	}
	variables := c.Variables
	if variables == nil {
		variables = []string{}
	}
	return Campaign{
		ID: c.ID.String(), Name: c.Name, AccountID: c.AccountID.String(), Channel: c.Channel, Status: c.Status,
		MessageBody: c.MessageBody, Variables: variables,
		MinIntervalSeconds: c.MinIntervalSeconds, JitterSeconds: c.JitterSeconds,
		Windows:         MapCampaignWindows(windows),
		ScheduleAt:      tsPtr(c.ScheduleAt),
		StartedAt:       tsPtr(c.StartedAt),
		CreatedBy:       c.CreatedBy.String(),
		CreatedAt:       c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       c.UpdatedAt.UTC().Format(time.RFC3339),
		RecipientCounts: counts,
	}
}

// CampaignTemplate is the API shape for one reusable message template (CAM-14).
type CampaignTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	MessageBody string   `json:"message_body"`
	Variables   []string `json:"variables"`
	IsArchived  bool     `json:"is_archived"`
	CreatedBy   string   `json:"created_by"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// MapCampaignTemplate maps a store.CampaignTemplate.
func MapCampaignTemplate(t store.CampaignTemplate) CampaignTemplate {
	variables := t.Variables
	if variables == nil {
		variables = []string{}
	}
	return CampaignTemplate{
		ID: t.ID.String(), Name: t.Name, MessageBody: t.MessageBody, Variables: variables,
		IsArchived: t.IsArchived, CreatedBy: t.CreatedBy.String(),
		CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: t.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// CampaignRecipient is the API shape for one persisted recipient row.
type CampaignRecipient struct {
	ID                 string            `json:"id"`
	CampaignID         string            `json:"campaign_id"`
	NormalizedIdentity string            `json:"normalized_identity"`
	RawInput           string            `json:"raw_input"`
	Name               string            `json:"name"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	Status             string            `json:"status"`
	FailureReason      string            `json:"failure_reason"`
	Attempts           int               `json:"attempts"`
	NextAttemptAt      *string           `json:"next_attempt_at"`
	ChatID             *string           `json:"chat_id"`
	MessageID          *string           `json:"message_id"`
	CreatedAt          string            `json:"created_at"`
	UpdatedAt          string            `json:"updated_at"`
	// MessageDeliveryState mirrors store.CampaignRecipient.MessageDeliveryState
	// — the linked message's own sent/delivered/read/failed, finer-grained
	// than Status above. Empty when MessageID is null.
	MessageDeliveryState string `json:"message_delivery_state"`
}

// MapCampaignRecipient maps a store.CampaignRecipient.
func MapCampaignRecipient(r store.CampaignRecipient) CampaignRecipient {
	return CampaignRecipient{
		ID: r.ID.String(), CampaignID: r.CampaignID.String(), NormalizedIdentity: r.NormalizedIdentity,
		RawInput: r.RawInput, Name: r.Name, Attributes: r.Attributes,
		Status: r.Status, FailureReason: r.FailureReason, Attempts: r.Attempts,
		NextAttemptAt: tsPtr(r.NextAttemptAt),
		ChatID:        nullUUIDPtr(r.ChatID), MessageID: nullUUIDPtr(r.MessageID),
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: r.UpdatedAt.UTC().Format(time.RFC3339),
		MessageDeliveryState: r.MessageDeliveryState,
	}
}

// CampaignEvent is the API shape for one campaign timeline entry.
type CampaignEvent struct {
	ID          string         `json:"id"`
	CampaignID  string         `json:"campaign_id"`
	Event       string         `json:"event"`
	ActorUserID *string        `json:"actor_user_id"`
	Detail      map[string]any `json:"detail,omitempty"`
	CreatedAt   string         `json:"created_at"`
}

// MapCampaignEvent maps a store.CampaignEvent.
func MapCampaignEvent(e store.CampaignEvent) CampaignEvent {
	return CampaignEvent{
		ID: e.ID.String(), CampaignID: e.CampaignID.String(), Event: e.Event,
		ActorUserID: nullUUIDPtr(e.ActorUserID), Detail: e.Detail,
		CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// CampaignTier is one rolling-window tier's shape — either configuration only
// (Used omitted) or, in a SendingBudget response, its live usage too.
type CampaignTier struct {
	WindowSeconds int  `json:"window_seconds"`
	MaxSends      int  `json:"max_sends"`
	Used          *int `json:"used,omitempty"`
}

// CampaignAccountSettings is the API shape for GET/PUT
// /accounts/:id/sending-limits.
type CampaignAccountSettings struct {
	AccountID          string           `json:"account_id"`
	LimitMode          string           `json:"limit_mode"`
	MinIntervalSeconds int              `json:"min_interval_seconds"`
	JitterSeconds      int              `json:"jitter_seconds"`
	Paused             bool             `json:"paused"`
	Tiers              []CampaignTier   `json:"tiers"`
	Windows            []CampaignWindow `json:"windows"`
}

// MapCampaignAccountSettings maps store.CampaignAccountSettings plus its
// sibling tier/window reads.
func MapCampaignAccountSettings(s store.CampaignAccountSettings, tiers []purecampaign.Tier, windows []store.CampaignWindow) CampaignAccountSettings {
	out := CampaignAccountSettings{
		AccountID: s.AccountID.String(), LimitMode: s.LimitMode,
		MinIntervalSeconds: s.MinIntervalSeconds, JitterSeconds: s.JitterSeconds, Paused: s.Paused,
		Windows: MapCampaignWindows(windows),
	}
	// Pre-allocated, never a bare append onto a nil slice: an unlimited
	// channel (the simulator has zero tiers) would otherwise serialize
	// "tiers": null and every client indexing it would fault.
	out.Tiers = make([]CampaignTier, 0, len(tiers))
	for _, t := range tiers {
		out.Tiers = append(out.Tiers, CampaignTier{WindowSeconds: t.WindowSeconds, MaxSends: t.MaxSends})
	}
	return out
}

// SendingBudget is the API shape for GET /accounts/:id/sending-budget — the
// live widget the Accounts page, the campaign wizard, and a campaign's own
// detail page all render from.
type SendingBudget struct {
	AccountID          string         `json:"account_id"`
	MinIntervalSeconds int            `json:"min_interval_seconds"`
	JitterSeconds      int            `json:"jitter_seconds"`
	Paused             bool           `json:"paused"`
	Allowed            bool           `json:"allowed"`
	ThrottledBy        int            `json:"throttled_by"`
	NextSendAt         string         `json:"next_send_at"`
	Tiers              []CampaignTier `json:"tiers"`
}

// MapSendingBudget maps a store.SendingBudget.
func MapSendingBudget(b store.SendingBudget) SendingBudget {
	out := SendingBudget{
		AccountID: b.AccountID.String(), MinIntervalSeconds: b.MinIntervalSeconds, JitterSeconds: b.JitterSeconds,
		Paused: b.Paused, Allowed: b.Allowed, ThrottledBy: b.ThrottledBy,
		NextSendAt: b.NextSendAt.UTC().Format(time.RFC3339),
	}
	// See MapCampaignAccountSettings: "tiers": null is what an unlimited
	// channel would otherwise serialize to.
	out.Tiers = make([]CampaignTier, 0, len(b.Tiers))
	for _, t := range b.Tiers {
		used := t.Used
		out.Tiers = append(out.Tiers, CampaignTier{WindowSeconds: t.WindowSeconds, MaxSends: t.MaxSends, Used: &used})
	}
	return out
}

// CampaignRecipientPreview is one row of a preview (parse-only, not yet
// persisted) response — the wire shape of backend/campaign.ParsedRecipient.
type CampaignRecipientPreview struct {
	Raw                string            `json:"raw"`
	Name               string            `json:"name,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	NormalizedIdentity string            `json:"normalized_identity,omitempty"`
	Status             string            `json:"status"`
	Reason             string            `json:"reason,omitempty"`
}

// CampaignRecipientPreviewResult is POST /campaigns/:id/preview's response
// shape — the wire shape of backend/campaign.ParseResult.
type CampaignRecipientPreviewResult struct {
	Rows      []CampaignRecipientPreview `json:"rows"`
	Total     int                        `json:"total"`
	Valid     int                        `json:"valid"`
	Invalid   int                        `json:"invalid"`
	Duplicate int                        `json:"duplicate"`
}

// CampaignStatusEvent is the campaign.status_changed SSE payload — ids and
// an enum status string only. internal/httpapi broadcasts this after every
// operator-initiated status change (start/pause/resume/stop); internal/
// campaign's Scheduler/Runner broadcast the identical shape for their own
// runtime transitions (auto-start, auto-complete, auto-pause) — ONE wire
// shape regardless of which side caused the change.
type CampaignStatusEvent struct {
	CampaignID string `json:"campaign_id"`
	Status     string `json:"status"`
}

// CampaignRecipientEvent is the campaign.recipient_updated SSE payload —
// ids and an enum status string only, never a recipient's raw_input/
// normalized_identity/name. Broadcast exclusively by internal/campaign's
// Runner as each send resolves.
type CampaignRecipientEvent struct {
	CampaignID  string `json:"campaign_id"`
	RecipientID string `json:"recipient_id"`
	Status      string `json:"status"`
}
