# 88Task - WhatsApp Task & Anti-Ban Safety Engine

High-performance task distribution, companion pairing, and messaging backend for WhatsApp task workflows with built-in Meta anti-ban safety and abuse prevention controls.

---

## 🛡️ Anti-Ban Safety Architecture

To protect connected WhatsApp accounts against spam flags, automated detection, and account bans, the engine implements a multi-layered defense pipeline across every stage of outbound message delivery:

### 1. Human-Like Presence & Typing Simulation
* **Presence Signal:** Dispatches a `ChatPresenceComposing` event to WhatsApp prior to delivery.
* **Randomized Typing Delay:** Simulates real human typing with a randomized delay between 2,000ms and 4,000ms (configurable via `typing_min_ms` and `typing_max_ms`).
* **Presence Cleanup:** Transitions to `ChatPresencePaused` before sending the payload.
* **Post-Send Cooldown & Chat Cleanup:** An optional post-send pause and automatic chat deletion (`delete_chat_after_send`) prevents recipients from flagging previous bulk message threads.

### 2. Spintax Message Variation
* **Dynamic Wording Rotation:** Evaluates spintax patterns such as `{Hi|Hello|Hey} {friend|there}` using nested expansion algorithms (`ResolveSpintax`).
* Prevents Meta spam filters from identifying identical text hashes across high-volume campaigns.

### 3. New Account Warm-Up Schedule
Gradually ramps outreach volume based on how long a WhatsApp account has been paired:
* **Day 1:** Max 5 messages / day
* **Day 2:** Max 10 messages / day
* **Day 3:** Max 15 messages / day
* **Day 4+:** Normal country-level daily limit

### 4. Pacing & Concurrency Controls
* **Per-Account Cooldown:** Enforces a randomized 45–90 second pause between consecutive sends on the same paired WhatsApp number.
* **PostgreSQL Advisory Locking:** Atomic locking (`pg_advisory_xact_lock`) scoped to the user account ID prevents race conditions and concurrent sends from parallel sessions.
* **Quota Management:**
  * Minimum send interval: 15 seconds
  * Max hourly messages: 20 / hour
  * Max daily messages: 100 / day

### 5. Recipient Pre-Validation (`IsOnWhatsApp`)
* Pre-checks recipient status via WhatsApp's interactive query before dispatching messages.
* Numbers not registered on WhatsApp are immediately marked `failed` and halted without attempting a send, eliminating dead-number abuse flags.

### 6. LID (Linked ID) Addressing & Caching
* Real WhatsApp Web companions communicate using Meta’s privacy-preserving LID (`@lid`) addresses rather than raw phone numbers.
* The system resolves and caches recipient LIDs, falling back gracefully to canonical phone mappings only when required.

### 7. Account Health & Automated Backoff
* **HTTP 429 (Rate-Limit) Detection:** Enforces an immediate 90-second cooldown on recipient lookups if WhatsApp returns rate-limit errors.
* **Status 463 (Timelock) Detection:** Pauses outreach automatically if WhatsApp flags the session with outreach restriction signals.

---

## ⚙️ Key Configuration Options

| Setting Key | Default | Description |
|---|---|---|
| `ban_safety_enabled` | `true` | Master switch for all ban safety checks |
| `send_typing` | `true` | Send composing presence before message |
| `typing_min_ms` | `2000` | Minimum simulated typing duration |
| `typing_max_ms` | `4000` | Maximum simulated typing duration |
| `safety_min_interval_ms` | `15000` | Minimum interval between consecutive messages |
| `safety_max_messages_hour` | `20` | Hourly safety threshold per account |
| `safety_max_messages_day` | `100` | Daily safety threshold per account |
| `delete_chat_after_send` | `true` | Automatically remove chat session from client |
