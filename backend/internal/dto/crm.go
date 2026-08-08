package dto

import (
	"encoding/json"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// Customer-management (CRM) wire shapes. Same rule as the rest of this
// package: one shape for the HTTP response and the SSE payload, so a Customer
// pushed over realtime is byte-identical to one from GET /customers/:id.

// CustomerStatus is one entry of the organization's configurable lifecycle.
type CustomerStatus struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Position  int    `json:"position"`
	IsDefault bool   `json:"is_default"`
}

// CustomerTag is one org-owned label.
type CustomerTag struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// CustomerIdentity is one channel identity. ExternalID is the provider's own
// identity for the person (phone JID / numeric Telegram user id) — the same
// value store.Contact.ExternalContactRef carries.
type CustomerIdentity struct {
	ID          string `json:"id"`
	Channel     string `json:"channel"`
	AccountID   string `json:"account_id"`
	ContactID   string `json:"contact_id"`
	ExternalID  string `json:"external_id"`
	Username    string `json:"username"`
	Phone       string `json:"phone"`
	DisplayName string `json:"display_name"`
}

// Customer is the unified person. There is no notes field: the profile's note
// is the latest CustomerNote, carried separately on CustomerProfile.
type Customer struct {
	ID             string             `json:"id"`
	DisplayName    string             `json:"display_name"`
	Phone          string             `json:"phone"`
	Email          string             `json:"email"`
	AvatarURL      string             `json:"avatar_url"`
	StatusID       *string            `json:"status_id"`
	Status         *CustomerStatus    `json:"status"`
	AssigneeUserID *string            `json:"assignee_user_id"`
	Tags           []CustomerTag      `json:"tags"`
	Identities     []CustomerIdentity `json:"identities"`
	CustomFields   map[string]any     `json:"custom_fields"`
	CreatedAt      string             `json:"created_at"`
	UpdatedAt      string             `json:"updated_at"`
}

// CustomerNote is one internal note. Nothing here is ever sent to a customer.
type CustomerNote struct {
	ID           string  `json:"id"`
	CustomerID   string  `json:"customer_id"`
	AuthorUserID *string `json:"author_user_id"`
	AuthorName   string  `json:"author_name"`
	Body         string  `json:"body"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// Followup is one scheduled next action. DueAt is the UTC instant everything
// orders and buckets by; DueDate/DueMinute are the wall clock the manager
// typed, so the edit form round-trips without a timezone drift (see
// 0013_crm.up.sql). DueMinute is null for an all-day follow-up.
type Followup struct {
	ID             string  `json:"id"`
	CustomerID     string  `json:"customer_id"`
	CustomerName   string  `json:"customer_name"`
	ConversationID *string `json:"conversation_id"`
	Channel        string  `json:"channel"`
	DueAt          string  `json:"due_at"`
	DueDate        string  `json:"due_date"`
	DueMinute      *int    `json:"due_minute"`
	Action         string  `json:"action"`
	Note           string  `json:"note"`
	AssigneeUserID *string `json:"assignee_user_id"`
	AssigneeName   string  `json:"assignee_name"`
	State          string  `json:"state"`
	CompletedAt    *string `json:"completed_at"`
	CreatedAt      string  `json:"created_at"`
}

// CustomerProfile is the sidebar's single hydrate call: the customer plus the
// pieces it always renders alongside them.
type CustomerProfile struct {
	Customer      Customer      `json:"customer"`
	LatestNote    *CustomerNote `json:"latest_note"`
	NextFollowup  *Followup     `json:"next_followup"`
	Conversations []Chat        `json:"conversations"`
}

// TimelineEntry is one row of the merged customer timeline. Source is "crm"
// for a recorded CRM event and "message" for conversation activity read live
// from the messages tables — the two have different fields populated, so the
// renderer switches on it rather than sniffing for nulls.
type TimelineEntry struct {
	ID          string         `json:"id"`
	Source      string         `json:"source"`
	Kind        string         `json:"kind"`
	ActorUserID *string        `json:"actor_user_id"`
	Summary     string         `json:"summary"`
	Detail      map[string]any `json:"detail,omitempty"`
	OccurredAt  string         `json:"occurred_at"`

	Channel    string  `json:"channel,omitempty"`
	ChatID     *string `json:"chat_id,omitempty"`
	Direction  string  `json:"direction,omitempty"`
	SenderKind string  `json:"sender_kind,omitempty"`
	Body       string  `json:"body,omitempty"`
}

// CustomFieldDef is one org-defined extra profile field.
type CustomFieldDef struct {
	ID        string   `json:"id"`
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	FieldType string   `json:"field_type"`
	Options   []string `json:"options"`
	Position  int      `json:"position"`
}

func MapCustomerStatus(st store.CustomerStatus) CustomerStatus {
	return CustomerStatus{
		ID: st.ID.String(), Slug: st.Slug, Name: st.Name, Color: st.Color,
		Position: st.Position, IsDefault: st.IsDefault,
	}
}

func MapCustomerTag(t store.CustomerTag) CustomerTag {
	return CustomerTag{ID: t.ID.String(), Slug: t.Slug, Name: t.Name, Color: t.Color}
}

func MapCustomerIdentity(ci store.CustomerIdentity) CustomerIdentity {
	return CustomerIdentity{
		ID: ci.ID.String(), Channel: ci.Channel, AccountID: ci.AccountID.String(),
		ContactID: ci.ContactID.String(), ExternalID: ci.ExternalID,
		Username: ci.Username, Phone: ci.Phone, DisplayName: ci.DisplayName,
	}
}

// MapCustomer maps a hydrated store.Customer. Tags and identities are emitted
// as empty arrays rather than null so the UI can iterate without a guard.
func MapCustomer(c store.Customer) Customer {
	tags := make([]CustomerTag, 0, len(c.Tags))
	for _, t := range c.Tags {
		tags = append(tags, MapCustomerTag(t))
	}
	ids := make([]CustomerIdentity, 0, len(c.Identities))
	for _, ci := range c.Identities {
		ids = append(ids, MapCustomerIdentity(ci))
	}
	out := Customer{
		ID:             c.ID.String(),
		DisplayName:    c.DisplayName,
		Phone:          c.Phone,
		Email:          c.Email,
		AvatarURL:      c.AvatarURL,
		StatusID:       nullUUIDPtr(c.StatusID),
		AssigneeUserID: nullUUIDPtr(c.AssigneeUserID),
		Tags:           tags,
		Identities:     ids,
		CustomFields:   decodeAttrs(c.CustomFields),
		CreatedAt:      c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      c.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if c.Status != nil {
		st := MapCustomerStatus(*c.Status)
		out.Status = &st
	}
	if out.CustomFields == nil {
		out.CustomFields = map[string]any{}
	}
	return out
}

func MapCustomerNote(n store.CustomerNote) CustomerNote {
	return CustomerNote{
		ID: n.ID.String(), CustomerID: n.CustomerID.String(),
		AuthorUserID: nullUUIDPtr(n.AuthorUserID), AuthorName: n.AuthorName, Body: n.Body,
		CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: n.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func MapFollowup(f store.Followup) Followup {
	return Followup{
		ID: f.ID.String(), CustomerID: f.CustomerID.String(), CustomerName: f.CustomerName,
		ConversationID: nullUUIDPtr(f.ConversationID), Channel: f.Channel,
		DueAt: f.DueAt.UTC().Format(time.RFC3339), DueDate: f.DueDate, DueMinute: f.DueMinute,
		Action: f.Action, Note: f.Note,
		AssigneeUserID: nullUUIDPtr(f.AssigneeUserID), AssigneeName: f.AssigneeName,
		State: f.State, CompletedAt: tsPtr(f.CompletedAt),
		CreatedAt: f.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func MapTimelineEntry(e store.TimelineEntry) TimelineEntry {
	out := TimelineEntry{
		ID: e.ID.String(), Source: e.Source, Kind: e.Kind,
		ActorUserID: nullUUIDPtr(e.ActorUserID), Summary: e.Summary,
		Detail:     decodeAttrs(e.Detail),
		OccurredAt: e.OccurredAt.UTC().Format(time.RFC3339),
	}
	if e.Source == store.TimelineSourceMessage {
		out.Channel = e.Channel
		out.ChatID = nullUUIDPtr(e.ChatID)
		out.Direction = e.Direction
		out.SenderKind = e.SenderKind
		out.Body = e.Body
	}
	return out
}

func MapCustomFieldDef(f store.CustomFieldDef) CustomFieldDef {
	opts := []string{}
	if len(f.Options) > 0 {
		// A malformed options blob degrades to "no choices" rather than failing
		// the whole list: the CHECK guarantees valid JSON, not a string array.
		_ = json.Unmarshal(f.Options, &opts)
	}
	return CustomFieldDef{
		ID: f.ID.String(), Key: f.Key, Label: f.Label, FieldType: f.FieldType,
		Options: opts, Position: f.Position,
	}
}
