# API Contracts

The authoritative contract for the HTTP surface: path conventions, the unified response envelope,
unified error codes, and the endpoint list. This supersedes the shorthand `/api/...` paths used
elsewhere in the docs.

## Path convention — service-prefixed, versioned

Every endpoint starts with the **service name** and a version:

- **xchats product API** (UI ↔ backend): `/xchats/api/v1/...`
- **Evolution webhook ingress** (Evolution → backend): `/evolution/api/v1/webhook/...`
- **Ops** (unversioned, no envelope): `/healthz`, `/readyz`, `/metrics`

So adding another transport later (e.g. Instagram) would mount under `/instagram/api/v1/webhook/...`
without touching the product API.

## Unified response envelope

**Every** xchats API response (success or error) is the same shape:

```json
{
  "payload": null,
  "errcode": "OK",
  "message": ""
}
```

- `payload` — the result data (object/array) on success; `null` on error.
- `errcode` — a machine code; **`"OK"` means success**, anything else is an error (see table).
- `message` — optional human-readable detail (for logs/debugging; never the source of truth).

The **HTTP status** reflects the category; `errcode` gives the precise machine reason. Clients
branch on `errcode`, not on parsing `message`.

Lists add paging inside `payload`:

```json
{ "payload": { "items": [], "page": 1, "page_size": 50, "total": 0 }, "errcode": "OK" }
```

The Evolution **webhook** endpoint also returns the envelope but Evolution only cares that it's a
fast `2xx`; on success it returns `{"payload":null,"errcode":"OK"}`.

## Unified error codes

| errcode | HTTP | Meaning |
|---|---|---|
| `OK` | 200/201 | success |
| `VALIDATION_ERROR` | 400 | malformed/invalid input |
| `UNAUTHORIZED` | 401 | missing/invalid session |
| `FORBIDDEN` | 403 | authenticated but not allowed (**unused in v1** — permissions are flat; reserved for a future `is_admin` gate on user-create / config-publish) |
| `NOT_FOUND` | 404 | resource does not exist |
| `CONFLICT` | 409 | duplicate / state conflict (e.g. instance name taken) |
| `RATE_LIMITED` | 429 | too many requests / send throttle hit |
| `WEBHOOK_UNAUTHORIZED` | 401 | webhook shared token missing/wrong |
| `ACCOUNT_NOT_ASSIGNED` | 409 | instance exists but isn't assigned to the org |
| `ACCOUNT_NOT_CONNECTED` | 409 | WhatsApp account not in `connected` state |
| `INSTANCE_NOT_FOUND` | 404 | Evolution has no such instance |
| `SEND_FAILED` | 502 | Evolution rejected/failed the send |
| `MEDIA_UNAVAILABLE` | 502 | media could not be fetched/stored |
| `EVOLUTION_ERROR` | 502 | generic upstream Evolution error |
| `AI_UNAVAILABLE` | 503 | LLM/assistant call failed |
| `EVAL_GATE_FAILED` | 409 | snapshot publish refused: quality gate not met (see `8.7-ai-evals.md`) |
| `INTERNAL` | 500 | unexpected server error |

New codes are added here only; the set is shared by backend and frontend.

## HTTP statuses and what they mean

The HTTP status is the **category**; `errcode` is the precise machine reason. They always agree.

| Status | Name | When | Typical errcode |
|---|---|---|---|
| 200 | OK | request succeeded; `payload` holds the result | `OK` |
| 201 | Created | a resource was created (e.g. add account, create user) | `OK` |
| 400 | Bad Request | malformed/invalid input (missing/invalid fields) | `VALIDATION_ERROR` |
| 401 | Unauthorized | **not authenticated** — no/expired/invalid session; or webhook token missing/wrong | `UNAUTHORIZED` / `WEBHOOK_UNAUTHORIZED` |
| 403 | Forbidden | **authenticated but not allowed** to do this/act on this resource | `FORBIDDEN` |
| 404 | Not Found | the resource/route doesn't exist | `NOT_FOUND` / `INSTANCE_NOT_FOUND` |
| 409 | Conflict | state/duplicate conflict (instance name taken, not assigned, not connected) | `CONFLICT` / `ACCOUNT_NOT_ASSIGNED` / `ACCOUNT_NOT_CONNECTED` |
| 422 | Unprocessable | well-formed but semantically invalid (fails a business rule) | `VALIDATION_ERROR` |
| 429 | Too Many Requests | rate limit / send throttle hit | `RATE_LIMITED` |
| 500 | Internal Server Error | unexpected server failure | `INTERNAL` |
| 502 | Bad Gateway | upstream failure (Evolution send/media error) | `SEND_FAILED` / `MEDIA_UNAVAILABLE` / `EVOLUTION_ERROR` |
| 503 | Service Unavailable | a dependency is down (LLM/assistant) | `AI_UNAVAILABLE` |

Quick guide to the three "permission" cases the UI must distinguish:
- **401 / `UNAUTHORIZED`** — you are not logged in (or the session expired) → send to login.
- **403 / `FORBIDDEN`** — you are logged in but lack rights for this action → show "not allowed".
- **409 / `ACCOUNT_NOT_ASSIGNED`** — the instance isn't assigned to the org, so we won't act on it → prompt to assign.

## Endpoints

### Auth & users — `/xchats/api/v1`

```text
POST   /xchats/api/v1/auth/login          {email, password} -> session
POST   /xchats/api/v1/auth/logout
GET    /xchats/api/v1/me                   current user + org
GET    /xchats/api/v1/users                list members
POST   /xchats/api/v1/users               {email, password} create member (joins default org)
```

### Organization — `/xchats/api/v1`

```text
GET    /xchats/api/v1/organization         org + settings
PATCH  /xchats/api/v1/organization        {auto_response_mode, auto_response_window, ...}
```

### WhatsApp accounts (simplified Evolution manager) — `/xchats/api/v1`

```text
GET    /xchats/api/v1/whatsapp-accounts                ALL instances + status + assigned flag
POST   /xchats/api/v1/whatsapp-accounts               {display_name, instance_name} add (create instance)
GET    /xchats/api/v1/whatsapp-accounts/{id}
POST   /xchats/api/v1/whatsapp-accounts/{id}/qr        (re)generate QR
GET    /xchats/api/v1/whatsapp-accounts/{id}/qr        poll QR / connection status
POST   /xchats/api/v1/whatsapp-accounts/{id}/assign
POST   /xchats/api/v1/whatsapp-accounts/{id}/unassign
```

### Inbox: conversations, messages, contacts — `/xchats/api/v1`

```text
GET    /xchats/api/v1/conversations                    list/filter
GET    /xchats/api/v1/conversations/{id}/messages
POST   /xchats/api/v1/conversations/{id}/messages      send text/media (-> outbound pipeline)
POST   /xchats/api/v1/conversations/{id}/assign        assign to a member
POST   /xchats/api/v1/conversations/{id}/read          mark read
GET    /xchats/api/v1/contacts
GET    /xchats/api/v1/contacts/{id}
GET    /xchats/api/v1/media/{id}                        stream stored media (resolves blob store)
```

### AI assistant — `/xchats/api/v1`

```text
# v1 — the draft loop:
POST   /xchats/api/v1/conversations/{id}/ai-drafts     Suggest: trigger a draft on demand (v1)
GET    /xchats/api/v1/conversations/{id}/ai-drafts
POST   /xchats/api/v1/ai-drafts/{id}/approve           approve -> send (idempotent; 409 on conflict/stale)

# deferred — Phase 4B (KB CMS):
GET    /xchats/api/v1/assistant/config                 persona/knowledge/prices/assets (published)
PUT    /xchats/api/v1/assistant/config                 edit draft config
POST   /xchats/api/v1/assistant/publish                publish a config version (eval-gated)
POST   /xchats/api/v1/assistant/playground             dry-run a draft (no send)
```

### Realtime — `/xchats/api/v1`

```text
GET    /xchats/api/v1/realtime                          SSE stream (message.*, conversation.*,
                                                         ai_draft.created, wa_account.status_changed)
```

### Evolution webhook ingress — `/evolution/api/v1`

```text
POST   /evolution/api/v1/webhook/{wa_account_id}            (+ /{event} subpaths if webhookByEvents)
```
Authenticated by the **single shared token** from `.env` (`WEBHOOK_UNAUTHORIZED` if missing/wrong).
Handler is store-raw + enqueue + `200` only.

### Ops (no version, no envelope)

```text
GET /healthz   GET /readyz   GET /metrics
```
