package dto_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// fakeCampaignRefsLookup is a minimal, hand-rolled double for the one
// method dto.MapChatWithCampaigns needs — no real database required to
// prove the mapping/attachment logic itself is correct.
type fakeCampaignRefsLookup struct {
	refs map[uuid.UUID][]store.CampaignRef
	err  error
}

func (f fakeCampaignRefsLookup) CampaignRefsForChats(_ context.Context, chatIDs []uuid.UUID) (map[uuid.UUID][]store.CampaignRef, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[uuid.UUID][]store.CampaignRef{}
	for _, id := range chatIDs {
		if refs, ok := f.refs[id]; ok {
			out[id] = refs
		}
	}
	return out, nil
}

// TestMapChatWithCampaigns_AttachesRefs is the direct regression guard for
// this feature's core UI requirement (a chat's campaign badge): a chat with
// persisted campaign participation must carry it through MapChatWithCampaigns,
// the ONE function every chat.updated/chat.created broadcast and chat-fetch
// response in the backend is required to route through instead of bare
// MapChat — see that function's own doc comment for the real regression
// (opening/assigning/resolving a campaign-linked chat used to broadcast a
// bare MapChat, silently erasing the badge from every other connected
// client's already-rendered row).
func TestMapChatWithCampaigns_AttachesRefs(t *testing.T) {
	chatID := uuid.New()
	campID := uuid.New()
	chat := store.Chat{ID: chatID}
	lookup := fakeCampaignRefsLookup{refs: map[uuid.UUID][]store.CampaignRef{
		chatID: {{ID: campID, Name: "Autumn Sale"}},
	}}

	out := dto.MapChatWithCampaigns(context.Background(), lookup, chat)

	if len(out.Campaigns) != 1 {
		t.Fatalf("expected 1 campaign ref, got %+v", out.Campaigns)
	}
	if out.Campaigns[0].ID != campID.String() || out.Campaigns[0].Name != "Autumn Sale" {
		t.Fatalf("unexpected campaign ref: %+v", out.Campaigns[0])
	}
}

// TestMapChatWithCampaigns_NoCampaignsIsOmittedNotEmpty confirms a chat with
// no campaign participation gets exactly what plain MapChat already
// produces (nil/omitted Campaigns) — never a spuriously-non-nil empty slice
// that would change the JSON shape (json:"campaigns,omitempty" only omits a
// nil slice, not an empty non-nil one).
func TestMapChatWithCampaigns_NoCampaignsIsOmittedNotEmpty(t *testing.T) {
	chat := store.Chat{ID: uuid.New()}
	lookup := fakeCampaignRefsLookup{refs: map[uuid.UUID][]store.CampaignRef{}}

	out := dto.MapChatWithCampaigns(context.Background(), lookup, chat)

	if out.Campaigns != nil {
		t.Fatalf("expected nil Campaigns for a chat with no participation, got %+v", out.Campaigns)
	}
}

// TestMapChatWithCampaigns_LookupFailureDegradesGracefully confirms a
// lookup error never fails the whole mapping — the attachment is a
// secondary enrichment, not a prerequisite, at every real call site (an
// HTTP response or a realtime broadcast alike).
func TestMapChatWithCampaigns_LookupFailureDegradesGracefully(t *testing.T) {
	chat := store.Chat{ID: uuid.New(), ChatState: "open"}
	lookup := fakeCampaignRefsLookup{err: context.DeadlineExceeded}

	out := dto.MapChatWithCampaigns(context.Background(), lookup, chat)

	if out.Campaigns != nil {
		t.Fatalf("expected nil Campaigns on lookup failure, got %+v", out.Campaigns)
	}
	if out.ID != chat.ID.String() || out.Status != "open" {
		t.Fatalf("expected the rest of the mapping to still succeed, got %+v", out)
	}
}
