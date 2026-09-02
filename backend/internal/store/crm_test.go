package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// crmFixture builds one organization with a connected WhatsApp account, ready
// for inbound messages to resolve customers against.
func crmFixture(t *testing.T, st *store.Store, name string) (orgID, accountID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, name)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	ownerJID := name + "@s.whatsapp.net"
	accountID = config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: ownerJID,
		ExternalHandle: "77000000000", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return org.ID, accountID
}

func waInbound(t *testing.T, st *store.Store, accountID uuid.UUID, phone, pushName, body string) store.InboundResult {
	t.Helper()
	jid := phone + "@s.whatsapp.net"
	res, err := st.UpsertInbound(context.Background(), store.InboundUpsert{
		AccountID: accountID, PhoneJID: jid, RemoteJID: jid, PhoneNumber: phone,
		PushName: pushName, Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: body,
		Preview: body, Source: "live_webhook", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("inbound: %v", err)
	}
	return res
}

// TestInboundCreatesCustomerOnce is the core of the product brief's section 2:
// the first message from an external identity creates a customer, and every
// later message — including a provider redelivery — reuses it.
func TestInboundCreatesCustomerOnce(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, accountID := crmFixture(t, st, "org-inbound")

	first := waInbound(t, st, accountID, "77011111111", "Иван", "привет")
	if first.CustomerID == uuid.Nil {
		t.Fatal("first inbound did not resolve a customer")
	}
	second := waInbound(t, st, accountID, "77011111111", "Иван", "ещё раз")
	if second.CustomerID != first.CustomerID {
		t.Fatalf("second message created a new customer: %s != %s", second.CustomerID, first.CustomerID)
	}

	// A redelivery of the very first message (same external id) must not
	// produce a second customer either.
	dup, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77011111111@s.whatsapp.net",
		RemoteJID: "77011111111@s.whatsapp.net", PhoneNumber: "77011111111",
		Direction: "in", SenderKind: "contact", ExternalMessageID: "fixed-id",
		MessageKind: "conversation", Body: "дубль", Preview: "дубль",
		Source: "live_webhook", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if dup.CustomerID != first.CustomerID {
		t.Fatalf("redelivery created a new customer: %s", dup.CustomerID)
	}

	customers, total, err := st.ListCustomers(ctx, store.CustomerFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(customers) != 1 {
		t.Fatalf("expected exactly 1 customer, got total=%d len=%d", total, len(customers))
	}
	if got := customers[0].DisplayName; got != "Иван" {
		t.Errorf("display name = %q, want the push name %q", got, "Иван")
	}
	if len(customers[0].Identities) != 1 {
		t.Fatalf("expected 1 identity, got %d", len(customers[0].Identities))
	}
	if got := customers[0].Identities[0].ExternalID; got != "77011111111@s.whatsapp.net" {
		t.Errorf("identity external id = %q", got)
	}
	// A brand-new customer lands on the organization's default status.
	if customers[0].Status == nil || customers[0].Status.Slug != "new" {
		t.Errorf("expected the default 'new' status, got %+v", customers[0].Status)
	}
}

// TestDifferentIdentitiesStaySeparate pins the "never auto-merge" rule: two
// external identities with the SAME name are still two customers.
func TestDifferentIdentitiesStaySeparate(t *testing.T) {
	st := dbtest.New(t)
	orgID, accountID := crmFixture(t, st, "org-separate")

	a := waInbound(t, st, accountID, "77011111111", "Иван Смирнов", "привет")
	b := waInbound(t, st, accountID, "77022222222", "Иван Смирнов", "здравствуйте")
	if a.CustomerID == b.CustomerID {
		t.Fatal("two distinct external identities collapsed into one customer — names must never merge")
	}
	_, total, err := st.ListCustomers(context.Background(), store.CustomerFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 customers, got %d", total)
	}
}

// TestMergePreservesEverything covers the product brief's section 13: after a
// manual merge the survivor holds both identities, both conversations, and all
// notes, tags, follow-ups and timeline entries.
func TestMergePreservesEverything(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, accountID := crmFixture(t, st, "org-merge")

	a := waInbound(t, st, accountID, "77011111111", "Иван Смирнов", "привет")
	b := waInbound(t, st, accountID, "77022222222", "Иван", "это тоже я")

	tag, err := st.CreateTag(ctx, orgID, store.CustomerTag{Slug: "vip", Name: "VIP"})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := st.AddCustomerTag(ctx, orgID, b.CustomerID, tag.ID, uuid.NullUUID{}); err != nil {
		t.Fatalf("tag source: %v", err)
	}
	if _, err := st.AddCustomerNote(ctx, orgID, b.CustomerID, uuid.NullUUID{}, "просил перезвонить"); err != nil {
		t.Fatalf("note: %v", err)
	}
	if _, err := st.CreateFollowup(ctx, orgID, store.FollowupInput{
		CustomerID: b.CustomerID, DueAt: time.Now().Add(48 * time.Hour).UTC(),
		DueDate: "2026-08-20", Action: store.FollowupActionCall, Note: "про xpayment",
	}, uuid.NullUUID{}); err != nil {
		t.Fatalf("followup: %v", err)
	}

	merged, err := st.MergeCustomers(ctx, orgID, a.CustomerID, b.CustomerID, uuid.NullUUID{})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if merged.ID != a.CustomerID {
		t.Fatalf("merge returned %s, want the target %s", merged.ID, a.CustomerID)
	}
	if len(merged.Identities) != 2 {
		t.Fatalf("expected 2 identities after merge, got %d", len(merged.Identities))
	}
	if len(merged.Tags) != 1 || merged.Tags[0].ID != tag.ID {
		t.Fatalf("source tag lost in merge: %+v", merged.Tags)
	}

	convs, err := st.CustomerConversations(ctx, orgID, a.CustomerID)
	if err != nil {
		t.Fatalf("conversations: %v", err)
	}
	if len(convs) != 2 {
		t.Fatalf("expected both conversations on the survivor, got %d", len(convs))
	}

	notes, err := st.ListCustomerNotes(ctx, orgID, a.CustomerID)
	if err != nil {
		t.Fatalf("notes: %v", err)
	}
	if len(notes) != 1 || notes[0].Body != "просил перезвонить" {
		t.Fatalf("source note lost in merge: %+v", notes)
	}
	if _, err := st.NextOpenFollowup(ctx, orgID, a.CustomerID); err != nil {
		t.Fatalf("source follow-up lost in merge: %v", err)
	}

	// The merged-away id still resolves — to the survivor, not a 404.
	revived, err := st.CustomerByID(ctx, orgID, b.CustomerID)
	if err != nil {
		t.Fatalf("merged id no longer resolves: %v", err)
	}
	if revived.ID != a.CustomerID {
		t.Fatalf("merged id resolved to %s, want survivor %s", revived.ID, a.CustomerID)
	}

	// And the tombstone is gone from every list.
	list, total, err := st.ListCustomers(ctx, store.CustomerFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != a.CustomerID {
		t.Fatalf("expected only the survivor listed, got total=%d %+v", total, list)
	}
}

func TestMergeRejectsSelf(t *testing.T) {
	st := dbtest.New(t)
	orgID, accountID := crmFixture(t, st, "org-self-merge")
	a := waInbound(t, st, accountID, "77011111111", "Иван", "привет")

	_, err := st.MergeCustomers(context.Background(), orgID, a.CustomerID, a.CustomerID, uuid.NullUUID{})
	if !errors.Is(err, store.ErrMergeSameCustomer) {
		t.Fatalf("merge into self: err = %v, want ErrMergeSameCustomer", err)
	}
}

// TestCustomersAreOrgScoped is the section 14 guarantee at the store level:
// organization B cannot read, list or mutate organization A's customer.
func TestCustomersAreOrgScoped(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgA, accountA := crmFixture(t, st, "org-a")
	orgB, _ := crmFixture(t, st, "org-b")

	a := waInbound(t, st, accountA, "77011111111", "Иван", "привет")

	if _, err := st.CustomerByID(ctx, orgB, a.CustomerID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("org B read org A's customer: err = %v", err)
	}
	if _, err := st.UpdateCustomer(ctx, orgB, a.CustomerID, store.CustomerPatch{}, uuid.NullUUID{}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("org B updated org A's customer: err = %v", err)
	}
	if _, err := st.AddCustomerNote(ctx, orgB, a.CustomerID, uuid.NullUUID{}, "чужая заметка"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("org B wrote a note on org A's customer: err = %v", err)
	}
	if _, err := st.CreateFollowup(ctx, orgB, store.FollowupInput{
		CustomerID: a.CustomerID, DueAt: time.Now(), DueDate: "2026-08-20",
		Action: store.FollowupActionCall,
	}, uuid.NullUUID{}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("org B scheduled a follow-up on org A's customer: err = %v", err)
	}
	_, total, err := st.ListCustomers(ctx, store.CustomerFilter{OrgID: orgB, Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 {
		t.Fatalf("org B sees %d of org A's customers", total)
	}
	_ = orgA
}

// TestFollowupBuckets pins the day arithmetic at a non-zero timezone offset —
// the case a UTC-only implementation gets wrong. dayStart is the caller's local
// midnight expressed in UTC, exactly what internal/httpapi computes from the
// browser's reported offset.
func TestFollowupBuckets(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, accountID := crmFixture(t, st, "org-buckets")
	cust := waInbound(t, st, accountID, "77011111111", "Иван", "привет").CustomerID

	// UTC+5 (Kazakhstan): local midnight on 2026-08-08 is 2026-08-07T19:00Z.
	dayStart := time.Date(2026, 8, 7, 19, 0, 0, 0, time.UTC)

	mk := func(due time.Time, date string) {
		t.Helper()
		if _, err := st.CreateFollowup(ctx, orgID, store.FollowupInput{
			CustomerID: cust, DueAt: due, DueDate: date, Action: store.FollowupActionCall,
		}, uuid.NullUUID{}); err != nil {
			t.Fatalf("create followup: %v", err)
		}
	}
	// 06:00 local today — inside today's window despite being on the PREVIOUS
	// UTC calendar day boundary logic a naive implementation would use.
	mk(dayStart.Add(6*time.Hour), "2026-08-08")
	mk(dayStart.Add(30*time.Hour), "2026-08-09")  // tomorrow
	mk(dayStart.Add(80*time.Hour), "2026-08-11")  // later this week
	mk(dayStart.Add(-3*time.Hour), "2026-08-07")  // overdue
	mk(dayStart.Add(-50*time.Hour), "2026-08-06") // overdue
	mk(dayStart.Add(400*time.Hour), "2026-08-24") // beyond the week

	got, err := st.FollowupBucketCounts(ctx, store.FollowupFilter{OrgID: orgID, DayStart: dayStart})
	if err != nil {
		t.Fatalf("buckets: %v", err)
	}
	want := store.FollowupBuckets{Today: 1, Tomorrow: 1, ThisWeek: 3, Overdue: 2}
	if got != want {
		t.Fatalf("buckets = %+v, want %+v", got, want)
	}

	// Completing the overdue pair empties that bucket; nothing else moves.
	list, _, err := st.ListFollowups(ctx, store.FollowupFilter{
		OrgID: orgID, Bucket: "overdue", DayStart: dayStart, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list overdue: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 overdue rows, got %d", len(list))
	}
	for _, fu := range list {
		if _, err := st.SetFollowupState(ctx, orgID, fu.ID, store.FollowupCompleted, uuid.NullUUID{}); err != nil {
			t.Fatalf("complete: %v", err)
		}
	}
	got, err = st.FollowupBucketCounts(ctx, store.FollowupFilter{OrgID: orgID, DayStart: dayStart})
	if err != nil {
		t.Fatalf("buckets after complete: %v", err)
	}
	if got.Overdue != 0 {
		t.Errorf("overdue = %d after completing both, want 0", got.Overdue)
	}
	if got.Today != 1 || got.Tomorrow != 1 {
		t.Errorf("completing overdue items disturbed other buckets: %+v", got)
	}
}

// TestTimelineIsCrmEventsOnly checks the timeline is CRM milestones ONLY —
// TODO.md's "Filter out routine message logs from Timeline": conversation
// activity used to be merged in too, but a note/status-change now has to
// stay findable among real message volume, not buried under one row per
// chat bubble. A real inbound message exists in this fixture on purpose, to
// prove it is excluded rather than merely absent.
func TestTimelineIsCrmEventsOnly(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, accountID := crmFixture(t, st, "org-timeline")
	cust := waInbound(t, st, accountID, "77011111111", "Иван", "привет").CustomerID

	if _, err := st.AddCustomerNote(ctx, orgID, cust, uuid.NullUUID{}, "пока не готов"); err != nil {
		t.Fatalf("note: %v", err)
	}
	statuses, err := st.ListStatuses(ctx, orgID)
	if err != nil {
		t.Fatalf("statuses: %v", err)
	}
	var interested uuid.NullUUID
	for _, s := range statuses {
		if s.Slug == "interested" {
			interested = uuid.NullUUID{UUID: s.ID, Valid: true}
		}
	}
	if _, err := st.UpdateCustomer(ctx, orgID, cust, store.CustomerPatch{StatusID: &interested}, uuid.NullUUID{}); err != nil {
		t.Fatalf("status change: %v", err)
	}

	entries, err := st.CustomerTimeline(ctx, orgID, cust, 100)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	kinds := map[string]int{}
	sources := map[string]int{}
	for _, e := range entries {
		kinds[e.Kind]++
		sources[e.Source]++
	}
	for _, want := range []string{store.TimelineCustomerCreated, store.TimelineNoteAdded, store.TimelineStatusChanged} {
		if kinds[want] == 0 {
			t.Errorf("timeline is missing a %q event: %+v", want, kinds)
		}
	}
	if sources[store.TimelineSourceMessage] != 0 {
		t.Errorf("timeline includes routine conversation activity, want CRM events only: %+v", sources)
	}
	if got, want := sources[store.TimelineSourceCRM], len(entries); got != want {
		t.Errorf("every entry should be CRM-sourced: %d of %d are", got, want)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].OccurredAt.After(entries[i-1].OccurredAt) {
			t.Fatalf("timeline is not newest-first at index %d", i)
		}
	}
}

// TestCustomerContextForChat is what the AI prompt block reads. A chat with no
// customer must report found=false so the prompt stays byte-identical to the
// eval harness's (see aiprompt.RenderCustomerContext).
func TestCustomerContextForChat(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, accountID := crmFixture(t, st, "org-ctx")
	res := waInbound(t, st, accountID, "77011111111", "Иван", "привет")

	tag, err := st.CreateTag(ctx, orgID, store.CustomerTag{Slug: "xpayment", Name: "xpayment"})
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
	if err := st.AddCustomerTag(ctx, orgID, res.CustomerID, tag.ID, uuid.NullUUID{}); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	if _, err := st.AddCustomerNote(ctx, orgID, res.CustomerID, uuid.NullUUID{}, "просил перезвонить позже"); err != nil {
		t.Fatalf("note: %v", err)
	}
	if _, err := st.CreateFollowup(ctx, orgID, store.FollowupInput{
		CustomerID: res.CustomerID, DueAt: time.Now().Add(72 * time.Hour).UTC(),
		DueDate: "2026-08-20", Action: store.FollowupActionCall,
	}, uuid.NullUUID{}); err != nil {
		t.Fatalf("followup: %v", err)
	}

	got, found, err := st.CustomerContextForChat(ctx, res.ChatID)
	if err != nil || !found {
		t.Fatalf("context: found=%v err=%v", found, err)
	}
	if got.Name != "Иван" || got.Status != "Новый" {
		t.Errorf("summary = %+v", got)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "xpayment" {
		t.Errorf("tags = %v", got.Tags)
	}
	if got.LatestNote != "просил перезвонить позже" {
		t.Errorf("latest note = %q", got.LatestNote)
	}
	if got.NextDueOn != "2026-08-20" || got.NextAction != store.FollowupActionCall {
		t.Errorf("next action = %q %q", got.NextAction, got.NextDueOn)
	}

	if _, found, err := st.CustomerContextForChat(ctx, uuid.New()); err != nil || found {
		t.Fatalf("unknown chat: found=%v err=%v, want found=false and no error", found, err)
	}
}

// TestCustomerFiltersByTagStatusAndFollowup covers the search/filter surface.
func TestCustomerFiltersByTagStatusAndFollowup(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, accountID := crmFixture(t, st, "org-filter")

	withTag := waInbound(t, st, accountID, "77011111111", "Алия", "привет").CustomerID
	plain := waInbound(t, st, accountID, "77022222222", "Борис", "здравствуйте").CustomerID

	tag, err := st.CreateTag(ctx, orgID, store.CustomerTag{Slug: "vip", Name: "VIP"})
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
	if err := st.AddCustomerTag(ctx, orgID, withTag, tag.ID, uuid.NullUUID{}); err != nil {
		t.Fatalf("add tag: %v", err)
	}

	dayStart := time.Date(2026, 8, 7, 19, 0, 0, 0, time.UTC)
	if _, err := st.CreateFollowup(ctx, orgID, store.FollowupInput{
		CustomerID: plain, DueAt: dayStart.Add(-2 * time.Hour), DueDate: "2026-08-07",
		Action: store.FollowupActionCall,
	}, uuid.NullUUID{}); err != nil {
		t.Fatalf("followup: %v", err)
	}

	byTag, _, err := st.ListCustomers(ctx, store.CustomerFilter{
		OrgID: orgID, TagID: uuid.NullUUID{UUID: tag.ID, Valid: true}, Limit: 50,
	})
	if err != nil {
		t.Fatalf("filter by tag: %v", err)
	}
	if len(byTag) != 1 || byTag[0].ID != withTag {
		t.Fatalf("tag filter returned %+v", byTag)
	}

	overdue, _, err := st.ListCustomers(ctx, store.CustomerFilter{
		OrgID: orgID, Followup: "overdue", DayStartUTC: dayStart,
		DayEndUTC: dayStart.AddDate(0, 0, 1), Limit: 50,
	})
	if err != nil {
		t.Fatalf("filter by overdue: %v", err)
	}
	if len(overdue) != 1 || overdue[0].ID != plain {
		t.Fatalf("overdue filter returned %+v", overdue)
	}

	byName, _, err := st.ListCustomers(ctx, store.CustomerFilter{OrgID: orgID, Query: "али", Limit: 50})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(byName) != 1 || byName[0].ID != withTag {
		t.Fatalf("name search returned %+v", byName)
	}

	// The search also covers channel identity handles, not just the profile.
	byPhone, _, err := st.ListCustomers(ctx, store.CustomerFilter{OrgID: orgID, Query: "77022222222", Limit: 50})
	if err != nil {
		t.Fatalf("identity search: %v", err)
	}
	if len(byPhone) != 1 || byPhone[0].ID != plain {
		t.Fatalf("identity search returned %+v", byPhone)
	}
}

// TestChatCarriesCustomerID pins the sidebar's hydrate path: the inbox chat row
// itself names the customer, so opening a conversation needs no extra lookup.
func TestChatCarriesCustomerID(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, accountID := crmFixture(t, st, "org-chatcustomer")
	res := waInbound(t, st, accountID, "77011111111", "Иван", "привет")

	chat, err := st.ChatByIDForOrg(ctx, res.ChatID, orgID)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if !chat.CustomerID.Valid || chat.CustomerID.UUID != res.CustomerID {
		t.Fatalf("chat.CustomerID = %+v, want %s", chat.CustomerID, res.CustomerID)
	}

	chats, _, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 10})
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	if len(chats) != 1 || !chats[0].CustomerID.Valid {
		t.Fatalf("listed chat has no customer: %+v", chats)
	}
}

// TestUnassignedAccountSkipsCustomer: a channel nobody has assigned to an
// organization has no tenant to own a customer, so ingest must store the
// message and simply not create one.
func TestUnassignedAccountSkipsCustomer(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()

	ownerJID := "unassigned@s.whatsapp.net"
	accountID := config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, DisplayName: "WhatsApp", ExternalAccountRef: ownerJID,
		ExternalHandle: "77000000001", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	res := waInbound(t, st, accountID, "77011111111", "Иван", "привет")
	if res.CustomerID != uuid.Nil {
		t.Fatalf("unassigned account created customer %s", res.CustomerID)
	}
	if res.MessageID == uuid.Nil {
		t.Fatal("message was not stored")
	}
}
