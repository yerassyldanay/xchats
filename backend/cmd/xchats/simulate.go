package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// simulateEnvelope mirrors httpapi's {payload, errcode, message} response
// envelope (server.go's envelope type) — this CLI is a thin HTTP client of the
// simulator API (10A): no DB, prompt, or LLM access of its own.
type simulateEnvelope struct {
	Payload json.RawMessage `json:"payload"`
	Errcode string          `json:"errcode"`
	Message string          `json:"message"`
}

// simulateDraft is the draft shape POST /simulator/messages returns when
// wait_for_response=true.
type simulateDraft struct {
	ID            string `json:"id"`
	Text          string `json:"text"`
	ReplyLanguage string `json:"reply_language"`
	Escalate      bool   `json:"escalate"`
}

type simulateResult struct {
	ConversationID string         `json:"conversation_id"`
	MessageID      string         `json:"message_id"`
	Draft          *simulateDraft `json:"draft"`
}

// runSimulateMessage implements `xchats simulate-message`: a thin HTTP client
// of POST /xchats/api/v1/simulator/messages (Phase 10A). It carries no
// database, prompt, or LLM access of its own — everything happens server-side.
func runSimulateMessage(args []string) {
	fs := flag.NewFlagSet("simulate-message", flag.ExitOnError)
	api := fs.String("api", "http://localhost:8080", "base URL of the running xchats API")
	token := fs.String("token", "", "authenticated session token (the xchats_session cookie value)")
	contact := fs.String("contact", "", "contact_ref: a stable, caller-chosen contact identifier")
	conversation := fs.String("conversation", "", "conversation_ref: a stable, caller-chosen conversation identifier")
	text := fs.String("text", "", "the message text to inject")
	wait := fs.Bool("wait", false, "wait for the generated draft (synchronous); omit for a fire-and-forget 202")
	provider := fs.String("provider", "", "override the LLM provider (must already be registered server-side)")
	model := fs.String("model", "", "override the LLM model (required together with -provider)")
	asJSON := fs.Bool("json", false, "print the raw JSON response instead of a human-readable summary")
	fs.Parse(args)

	if *token == "" || *contact == "" || *conversation == "" || *text == "" {
		fmt.Fprintln(os.Stderr, "simulate-message: -token, -contact, -conversation, and -text are all required")
		os.Exit(2)
	}
	if (*provider == "") != (*model == "") {
		fmt.Fprintln(os.Stderr, "simulate-message: -provider and -model must both be set, or both omitted")
		os.Exit(2)
	}

	body := map[string]any{
		"contact_ref": *contact, "conversation_ref": *conversation, "text": *text,
		"wait_for_response": *wait,
	}
	if *provider != "" {
		body["provider"] = *provider
		body["model"] = *model
	}
	raw, err := json.Marshal(body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "simulate-message: encode request:", err)
		os.Exit(1)
	}

	url := strings.TrimRight(*api, "/") + "/xchats/api/v1/simulator/messages"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		fmt.Fprintln(os.Stderr, "simulate-message: build request:", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "xchats_session="+*token)

	// 150s covers the engine's own worst case (~2 min: two 60s LLM attempts)
	// plus network/DB overhead, with headroom.
	client := &http.Client{Timeout: 150 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "simulate-message: request failed:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "simulate-message: read response:", err)
		os.Exit(1)
	}

	var env simulateEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		fmt.Fprintf(os.Stderr, "simulate-message: unparseable response (status %d): %s\n", resp.StatusCode, string(respBody))
		os.Exit(1)
	}
	if resp.StatusCode >= 300 || (env.Errcode != "" && env.Errcode != "OK") {
		fmt.Fprintf(os.Stderr, "simulate-message: %s (status %d, code %s)\n", env.Message, resp.StatusCode, env.Errcode)
		os.Exit(1)
	}

	if *asJSON {
		fmt.Println(string(env.Payload))
		return
	}
	var result simulateResult
	if err := json.Unmarshal(env.Payload, &result); err != nil {
		fmt.Fprintln(os.Stderr, "simulate-message: unparseable payload:", err)
		os.Exit(1)
	}
	fmt.Printf("conversation_id: %s\n", result.ConversationID)
	fmt.Printf("message_id:      %s\n", result.MessageID)
	if result.Draft == nil {
		fmt.Println("draft:           (not requested — pass -wait to generate one synchronously)")
		return
	}
	fmt.Printf("draft_id:        %s\n", result.Draft.ID)
	fmt.Printf("reply_language:  %s\n", result.Draft.ReplyLanguage)
	fmt.Printf("escalate:        %v\n", result.Draft.Escalate)
	fmt.Printf("text:            %s\n", result.Draft.Text)
}
