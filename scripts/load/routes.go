package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync/atomic"
)

// Route names double as both the stats-collector key and the weighted-mix
// selector key.
const (
	routeDebugWAEvent = "debug_wa_event"
	routeListChats    = "list_chats"
	routeListMessages = "list_messages"
	routeKB           = "kb"
	routeSimulator    = "simulator_message"
	routeSendMessage  = "send_message"
)

// defaultWeights is the spec's own default request mix, in percent —
// callers may override via --weights.
var defaultWeights = map[string]float64{
	routeDebugWAEvent: 25,
	routeListChats:    15,
	routeListMessages: 15,
	routeKB:           20,
	routeSimulator:    20,
	routeSendMessage:  5,
}

// weightedRoutes turns a weight map into a cumulative-distribution table
// for O(n) selection — built once per run, read-only thereafter (safe for
// concurrent use by every worker).
type weightedRoutes struct {
	names []string
	cum   []float64 // cumulative weight, same length/order as names
	total float64
}

func newWeightedRoutes(weights map[string]float64) (*weightedRoutes, error) {
	w := &weightedRoutes{}
	var total float64
	for name, weight := range weights {
		if weight < 0 {
			return nil, fmt.Errorf("weight for %q is negative", name)
		}
		if weight == 0 {
			continue
		}
		total += weight
		w.names = append(w.names, name)
		w.cum = append(w.cum, total)
	}
	if total <= 0 {
		return nil, fmt.Errorf("request mix has no positive weight at all")
	}
	w.total = total
	return w, nil
}

func (w *weightedRoutes) Pick(rng *rand.Rand) string {
	r := rng.Float64() * w.total
	for i, c := range w.cum {
		if r < c {
			return w.names[i]
		}
	}
	return w.names[len(w.names)-1]
}

// runner bundles everything one worker iteration needs to execute any
// route — built once in main, shared read-only (its own fields are each
// individually thread-safe) across every worker goroutine.
type runner struct {
	client    *apiClient
	chats     *reservoir
	byChannel *channelReservoirs
	contacts  *contactPool
	extSeq    int64 // atomic counter for unique debug/wa-event external ids
}

// execute runs one iteration of route, using rng for this worker's own
// request-parameter choices (never for anything requiring cross-worker
// synchronization — that lives inside the reservoirs themselves). workerID
// only feeds unique-id generation. Returns the HTTP status actually
// observed (0 if the request never got a response at all) alongside any
// error, so callers can tally both independently.
func (r *runner) execute(ctx context.Context, route string, rng *rand.Rand, workerID int) (int, error) {
	switch route {
	case routeDebugWAEvent:
		return r.debugWAEvent(ctx, rng, workerID)
	case routeListChats:
		return r.listChats(ctx)
	case routeListMessages:
		return r.listMessages(ctx, rng)
	case routeKB:
		return r.getKB(ctx)
	case routeSimulator:
		return r.simulatorMessage(ctx, rng)
	case routeSendMessage:
		return r.sendMessage(ctx, rng)
	default:
		return 0, fmt.Errorf("unknown route %q", route)
	}
}

type debugWAEventResp struct {
	OK        bool   `json:"ok"`
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id"`
}

func (r *runner) debugWAEvent(ctx context.Context, rng *rand.Rand, workerID int) (int, error) {
	seq := atomic.AddInt64(&r.extSeq, 1)
	sender := r.contacts.Pick(rng)
	var out debugWAEventResp
	status, err := r.client.do(ctx, http.MethodPost, "/xchats/api/v1/debug/wa-event", map[string]any{
		"event_type":  "message",
		"sender_jid":  sender,
		"text":        "Здравствуйте! Сколько стоит доставка?",
		"external_id": fmt.Sprintf("load-%d-%d", workerID, seq),
	}, &out)
	if err != nil {
		return status, err
	}
	r.chats.Add(out.ChatID)
	r.byChannel.Add("whatsapp", out.ChatID)
	return status, nil
}

type chatItem struct {
	ID      string `json:"id"`
	Channel string `json:"channel"`
}
type listChatsResp struct {
	Items []chatItem `json:"items"`
}

func (r *runner) listChats(ctx context.Context) (int, error) {
	var out listChatsResp
	status, err := r.client.do(ctx, http.MethodGet, "/xchats/api/v1/chats?page_size=50", nil, &out)
	if err != nil {
		return status, err
	}
	for _, c := range out.Items {
		r.chats.Add(c.ID)
		r.byChannel.Add(c.Channel, c.ID)
	}
	return status, nil
}

func (r *runner) listMessages(ctx context.Context, rng *rand.Rand) (int, error) {
	chatID, ok := r.chats.Pick(rng)
	if !ok {
		return 0, nil // nothing discovered yet — not a failure, see reservoir's own doc comment
	}
	var out struct {
		Items []json.RawMessage `json:"items"`
	}
	return r.client.do(ctx, http.MethodGet, "/xchats/api/v1/chats/"+chatID+"/messages?limit=50", nil, &out)
}

func (r *runner) getKB(ctx context.Context) (int, error) {
	return r.client.do(ctx, http.MethodGet, "/xchats/api/v1/kb", nil, nil)
}

type simulatorResp struct {
	ConversationID string `json:"conversation_id"`
}

func (r *runner) simulatorMessage(ctx context.Context, rng *rand.Rand) (int, error) {
	contact := r.contacts.Pick(rng)
	var out simulatorResp
	status, err := r.client.do(ctx, http.MethodPost, "/xchats/api/v1/simulator/messages", map[string]any{
		"contact_ref":       contact,
		"conversation_ref":  contact,
		"text":              "Здравствуйте! Сколько стоит доставка?",
		"wait_for_response": true,
	}, &out)
	if err != nil {
		return status, err
	}
	r.chats.Add(out.ConversationID)
	r.byChannel.Add("simulator", out.ConversationID)
	return status, nil
}

func (r *runner) sendMessage(ctx context.Context, rng *rand.Rand) (int, error) {
	channel, ok := r.byChannel.NextChannel()
	var chatID string
	if ok {
		chatID, ok = r.byChannel.Pick(rng, channel)
	}
	if !ok {
		chatID, ok = r.chats.Pick(rng)
	}
	if !ok {
		return 0, nil // nothing discovered yet
	}
	return r.client.do(ctx, http.MethodPost, "/xchats/api/v1/chats/"+chatID+"/messages", map[string]any{
		"text": "Mock load-test outbound message",
	}, nil)
}

// Preflight validates every route once, with real parameters, before the
// timed run starts — "validate each route before starting load." It also
// seeds the reservoirs off GET /chats so list_messages/send_message have
// something to pick from as soon as measurement begins.
func (r *runner) Preflight(ctx context.Context, rng *rand.Rand) error {
	if _, err := r.listChats(ctx); err != nil {
		return fmt.Errorf("preflight GET /chats: %w", err)
	}
	if r.chats.Len() == 0 {
		return fmt.Errorf("preflight: GET /chats returned no chats — seed demo data first (make profile-server does this automatically)")
	}
	if _, err := r.getKB(ctx); err != nil {
		return fmt.Errorf("preflight GET /kb: %w", err)
	}
	if _, err := r.listMessages(ctx, rng); err != nil {
		return fmt.Errorf("preflight GET /chats/:id/messages: %w", err)
	}
	if _, err := r.debugWAEvent(ctx, rng, -1); err != nil {
		return fmt.Errorf("preflight POST /debug/wa-event: %w", err)
	}
	if _, err := r.simulatorMessage(ctx, rng); err != nil {
		return fmt.Errorf("preflight POST /simulator/messages: %w", err)
	}
	if _, err := r.sendMessage(ctx, rng); err != nil {
		return fmt.Errorf("preflight POST /chats/:id/messages: %w", err)
	}
	return nil
}
