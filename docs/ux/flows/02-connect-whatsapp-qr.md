# Connect WhatsApp (QR Scan) — User Flow

> **Purpose:** Trace exactly what a user sees and does when connecting a WhatsApp
> number via QR code (whatsmeow). Friction points are marked with 🔴.

---

## How the User Gets Here

The user must first navigate to `/accounts` (the "Channels" icon in the nav rail).
There are two entry points:

1. **From the nav rail:** User clicks the Radio icon labeled "Channels"
2. **From the Setup Wizard:** User clicks "Go to Channels" button on wizard step 2/3

---

## User Flow Diagram

```mermaid
flowchart TD
    Start["User clicks 'Channels' in nav rail"]
    Start --> AccountsPage

    subgraph AccountsPage["Screen: /accounts"]
        direction TB
        PageSees["User sees:
        • Header: 'Channels'
        • 3 stat cards: Connected 0 | Waiting on action 0 | Not connected 0
        • Empty state: WhatsApp + Telegram icons
        • Text: 'No channels connected yet'
        • Button: '+ Connect a channel'"]
    end

    AccountsPage -->|"Clicks '+ Connect a channel'"| PickerModal

    subgraph PickerModal["Modal: Connect a channel"]
        direction TB
        PickerSees["User sees 5 channel cards in a grid:

        WhatsApp — QR code pairing
        01 Open WhatsApp on your phone
        02 Go to Linked Devices
        03 Scan the QR code

        Telegram — Paste BotFather token
        WhatsApp Cloud — Official Cloud API
        Instagram — Connect via Meta
        Messenger — Connect via Meta

        Footer: QR/token storage and Meta setup note"]
    end

    PickerModal -->|"Clicks WhatsApp card"| QRReady

    subgraph QRReady["Modal: QR Pairing"]
        direction TB
        QRSees["User sees:
        • Title: Add a WhatsApp number
        • Instructions: Open WhatsApp → Linked Devices → Link a Device
        • A spinner occupies the 208x208 QR card until the first code arrives
        • QR code image in a bordered card once available
        • Status: The code refreshes automatically. Waiting for scan…
        • Frontend polls the backend every 2.5 seconds"]
    end

    QRReady --> ScanDecision{"User scans QR\nwith phone?"}

    ScanDecision -->|"Scan succeeds"| SuccessModal
    ScanDecision -->|"Backend QR channel reports timeout"| TimeoutState
    ScanDecision -->|"Backend restarts mid-scan"| SessionExpired

    subgraph TimeoutState["Modal: Timeout Error"]
        direction TB
        TimeoutSees["User sees:
        • Red icon + backend message or 'The wait timed out.'
        • QR image disappears entirely
        • Button: Try again"]
    end
    TimeoutState -->|"Clicks Try again"| QRReady

    subgraph SessionExpired["Modal: Session Lost"]
        direction TB
        ExpiredSees["User sees:
        • Red icon + 'The connection session expired. Please try again.'
        • No explanation of why
        • Button: Try again"]
    end
    SessionExpired -->|"Clicks Try again"| QRReady

    subgraph SuccessModal["Modal: Success — auto-closes in 900ms"]
        direction TB
        SuccessSees["User sees:
        • Large green checkmark
        • Text: 'Number connected!'
        • Dialog closes automatically"]
    end

    SuccessModal --> FinalState

    subgraph FinalState["Screen: /accounts — Updated"]
        direction TB
        FinalSees["User sees:
        • Stat cards: Connected 1
        • New account card with:
          - WhatsApp icon tile
          - Phone number
          - Green 'Connected' badge
          - Automation status badge
          - Icon-only buttons: Automation | Delete"]
    end
```

---

## Friction Points and Suggested Changes

### 🔴 1. No Warning About Phone Requirements Before Starting

**What happens today:** User clicks the WhatsApp card and the dialog starts a pairing session immediately, showing a spinner in the QR card until the first code arrives.
If their phone has no internet, WhatsApp is outdated, or they already linked the
maximum number of devices — the scan silently fails and eventually times out.

**Suggested change:** Before showing QR, display a pre-flight checklist:
- Make sure your phone has internet connection
- WhatsApp must be updated to latest version
- You can link up to 4 devices per number

---

### 🔴 2. Pairing Timeout Gives No Time Expectation or Recovery Context

**What happens today:** The frontend has no fixed 60-second timer or visible countdown. Whatsmeow rotates QR codes and eventually reports a backend-owned timeout; when that happens, the QR disappears and the user sees the backend message or the fallback "The wait timed out." They are not told how long pairing remains available or that codes refresh automatically before the terminal timeout.

**Suggested change:** Show a more helpful message:
"The pairing session ended before the phone linked. Click Try again to start a fresh session; QR codes refresh automatically while the session is active."

---

### 🔴 3. Session Expired Error is Cryptic

**What happens today:** If the backend restarts during pairing, the poll returns 404 and the user sees "The connection session expired. Please try again." The copy gives the next action but not the likely cause.

**Suggested change:** Explain: "The pairing session was lost (the server may
have restarted). Click Retry to start a new session — no data was lost."

---

### 🔴 4. Success Auto-Closes Too Fast (900ms)

**What happens today:** The success modal shows for only 900ms. On a slow render or
if the user blinks, they miss the confirmation entirely and wonder if it worked.

**Suggested change:** Either extend to 2–3 seconds, or do not auto-close — let the
user click a "Done" button to acknowledge.

---

### 🔴 5. No "What's Next?" After Connecting

**What happens today:** After the modal closes, the user is back on `/accounts` with
a new card. There is no guidance on what to do next.

**Suggested change:** Show a one-time banner after first channel connection:
"WhatsApp connected! Next step: add content to your Knowledge Base so the
assistant can start drafting replies. [Go to Knowledge Base →]"

---

### 🔴 6. Reconnect Flow is Hidden When Connection Drops

**What happens today:** If the WhatsApp session drops later (phone lost, WhatsApp
logged out), the card shows a broken status. The only way to reconnect is a
small unlabeled icon button (RotateCw) that is easy to miss.

**Suggested change:** Show a prominent yellow banner on the card:
"Connection lost — your WhatsApp session expired. [Reconnect via QR →]"

---

## Source Components

| Element | File |
|---|---|
| Channels page | [`Accounts.vue`](../../../frontend/src/views/Accounts.vue) |
| Channel picker + QR dialog | [`AddAccountDialog.vue`](../../../frontend/src/components/AddAccountDialog.vue) |
| Account cards and empty state | [`Accounts.vue` L284–418](../../../frontend/src/views/Accounts.vue#L284-L418) |
| Automation dialog | [`AutomationSettingsDialog.vue`](../../../frontend/src/components/AutomationSettingsDialog.vue) |
| Accounts store | [`stores/accounts.ts`](../../../frontend/src/stores/accounts.ts) |
| Backend QR event handling | [`pairing.go`](../../../backend/internal/whatsmeow/pairing.go) |
