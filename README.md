# Silon Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/KUWAITNET/silon-go-sdk.svg)](https://pkg.go.dev/github.com/KUWAITNET/silon-go-sdk)

Go client for the [Silon](https://silon.tech) messaging platform API — send
messages on any channel (WhatsApp, SMS, email, push, web push, voice), manage
CRM contacts and groups, run bulk campaigns, maintain the do-not-contact
list, consume events, and verify webhooks. Stdlib only — zero third-party
dependencies.

## Installation

```bash
go get github.com/KUWAITNET/silon-go-sdk
```

Requires Go 1.24+. The module depends only on the Go standard library.

```go
import "github.com/KUWAITNET/silon-go-sdk"
```

## Quickstart

```go
client, err := silon.NewClient(
    silon.WithAPIKey("sk_live_..."), // Settings → API keys; or set SILON_API_KEY
    silon.WithWorkspace("acme"),     // => https://acme.silon.tech; or SILON_WORKSPACE / SILON_BASE_URL
)
if err != nil {
    log.Fatal(err)
}

sent, err := client.Messages.Send(ctx, silon.MessageSendParams{
    Channel: "whatsapp",
    To:      map[string]any{"client_id": "cust_001"},
    Content: map[string]any{"body": "Your order has shipped"},
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(sent.ID, sent.Status) // e.g. "9f3e..." "queued"
```

There is one synchronous client; it is safe for concurrent use — run calls
from goroutines and cancel them via the `context.Context` every method takes.

## Sending

One endpoint, every channel. `To` targets a single recipient; `Audience`
fans out a broadcast — exactly one of the two is required (a client-side
`*silon.Error` is returned otherwise).

```go
// Approved WhatsApp template to a raw number
client.Messages.Send(ctx, silon.MessageSendParams{
    Channel: "whatsapp",
    To:      map[string]any{"phone_number": "+12025550123"},
    WhatsAppTemplate: map[string]any{
        "name": "order_confirmed", "language": "en",
        "variables": map[string]any{"body_1": "Sara", "body_2": "ORD-42"},
    },
    Provider: silon.String("meta_cloud"),
})
```

Look up delivery status with `Messages.Retrieve`. The modern shape carries
`ID`, `Object`, `Channel` (nullable) and a typed `Timeline` — the ordered
status transitions (`{Status, At, Provider}`, ascending by `At`). A single
send's `Timeline` runs `queued → sent → delivered` when the channel reports
receipts; `delivered` appears **only** in the timeline, never as the
top-level `Status`:

```go
status, err := client.Messages.Retrieve(ctx, sent.ID)
for _, step := range status.Timeline {
    fmt.Println(step.Status, step.At) // queued, sent, delivered, ...
}
```

The legacy `EventID`, `IsSent` and per-recipient `Messages` fields are
still populated but **deprecated** (gopls / staticcheck flag them) — prefer
`ID`, `Status` and `Timeline`.

## Batches

Many independent, personalised messages in one call — use it when every
recipient gets *different* content (for one content fanned out to an
audience, use a broadcast). Exactly one of `Messages` (up to 500 inline
rows) or `File` (a saved CSV name) is required — a client-side
`*silon.Error` is returned otherwise. Request-level fields (`Channel`,
`Content`, `Template`, `Provider`, ...) are row defaults on both forms; a
row's own field (or CSV column) always wins.

Inline rows are free-form maps with the same shape as a single send minus
`audience`:

```go
batch, err := client.Messages.SendBatch(ctx, silon.MessageBatchParams{
    Channel: silon.String("sms"),
    Messages: []map[string]any{
        {"to": map[string]any{"phone_number": "+96550001234"},
            "content": map[string]any{"body": "Sara, your table for 2 is confirmed."}},
        {"to": map[string]any{"phone_number": "+96550001235"},
            "content": map[string]any{"body": "Omar, your table for 4 is confirmed."}},
    },
})
for _, row := range batch.Messages { // request order
    fmt.Println(row.ID, row.Status)  // each ID works with Messages.Retrieve
}
```

Validation is all-or-nothing: every row is validated up front and any
invalid row 422s the whole batch with a per-index `Attr` like
`messages[3].to.phone_number` — nothing is queued. An empty list is a 422
`batch-empty`; more than 500 rows is a 422 `batch-too-large`. Inline
batches have no GET endpoint — the per-row message ids are the tracking
primitive. Requires the `messages:send` scope.

For unbounded row counts, upload a CSV once and send it by name — rows
expand asynchronously, so the 202 is the aggregate envelope only
(`Messages` is nil):

```go
upload, err := client.Bulk.Files.Upload(ctx, silon.BulkFileUploadParams{
    Path: "recipients.csv", // e.g. name,phone_number columns
})
batch, err := client.Messages.SendBatch(ctx, silon.MessageBatchParams{
    File:    silon.String(upload.Name),
    Channel: silon.String("sms"),
    Content: map[string]any{"body": "Hello {{name}}"}, // {{columns}} render per row
})
fmt.Println(batch.ID, batch.Status) // "queued"; RowCount when cheaply known
```

The file-form `batch.ID` is the created bulk batch id — read per-row
status with `client.Bulk.Retrieve(ctx, id)` and the bulk reports. A
heterogeneous CSV (per-row `channel` / `message` columns) needs no
defaults at all. An unknown `File` name is a 404 `file-not-found`;
defaults the bulk pipeline cannot honor are rejected with a 422
`batch-invalid`. This one endpoint replaces the deprecated `Bulk.Send`
for every file shape.

## Broadcasts

One piece of content fanned out to an audience — a CRM group, explicit
client ids, or an inline ad-hoc list of raw addresses (max 1,000 rows;
duplicates are deduped, suppressed recipients skipped — `SkippedCount` is
the total and `Skipped` itemises it per reason):

```go
// Email broadcast to a client group
result, err := client.Broadcasts.Create(ctx, silon.BroadcastCreateParams{
    Channel:  "email",
    Audience: map[string]any{"type": "client_group", "slug": "vip"},
    Content:  map[string]any{"subject": "We saved you a seat", "body": "<h1>Hello</h1>"},
})
fmt.Println(result.TargetCount, result.SkippedCount)
if result.Skipped != nil { // additive breakdown; SkippedCount stays the sum
    fmt.Println(result.Skipped.Suppressed, result.Skipped.WrongChannel, result.Skipped.Duplicate)
}

// SMS to an ad-hoc recipient list
client.Broadcasts.Create(ctx, silon.BroadcastCreateParams{
    Channel: "sms",
    Audience: map[string]any{
        "type": "recipients",
        "recipients": []any{
            map[string]any{"phone_number": "+96550001234"},
            map[string]any{"phone_number": "+96550001235"},
            map[string]any{"client_id": "cust_001"},
        },
    },
    Content: map[string]any{"body": "Flash sale ends tonight"},
})

// Track it
broadcast, err := client.Broadcasts.Retrieve(ctx, result.ID)
page, err := client.Broadcasts.Deliveries(ctx, result.ID, silon.BroadcastDeliveriesParams{})
for delivery, err := range page.All(ctx) {
    if err != nil {
        return err
    }
    fmt.Println(delivery.ClientID, delivery.Status)
}
```

Requires the `broadcasts:send` scope. (`Messages.Send` with an `Audience`
keeps working as a legacy alias for the same fan-out.)

Every `Messages.Send` / `Messages.SendBatch` / `Broadcasts.Create` /
`OTP.Send` call carries an `Idempotency-Key` header (auto-generated UUIDv4
unless you set `IdempotencyKey`), so automatic retries can never
double-send.

Optional scalar params are pointers — use the helpers `silon.String`,
`silon.Int`, `silon.Bool`, `silon.Float`, `silon.Time`. Fields this SDK
version does not model can be passed via `ExtraBody`, merged into the
request body last.

## Scheduling and cancellation

`Messages.Send`, `Broadcasts.Create`, and the *file* form of
`Messages.SendBatch` take an optional `SendAt` (`*time.Time`, serialized
ISO-8601 with the value's own UTC offset). The server requires it to be
strictly in the future and at most 90 days ahead — otherwise a 422
`send-at-invalid`. A scheduled create answers the normal `202` envelope
with status `"scheduled"`:

```go
sent, err := client.Messages.Send(ctx, silon.MessageSendParams{
    Channel: "sms",
    To:      map[string]any{"phone_number": "+96550001234"},
    Content: map[string]any{"body": "Doors open in one hour."},
    SendAt:  silon.Time(time.Now().Add(24 * time.Hour)),
})
fmt.Println(sent.Status) // "scheduled"
```

The envelope `ID` is stable across the lifecycle — it resolves via
`Messages.Retrieve` / `Broadcasts.Retrieve` before *and* after dispatch
(`scheduled`, then the normal queued/sent lifecycle; the status endpoints
also report `SendAt`). While a send is still scheduled, cancel it:

```go
canceled, err := client.Messages.Cancel(ctx, sent.ID)
fmt.Println(canceled.Status) // "canceled"

// Same shape for broadcasts:
canceled2, err := client.Broadcasts.Cancel(ctx, broadcast.ID)
```

A canceled send never dispatches and emits a `message.canceled` /
`broadcast.canceled` event. Cancel is idempotent by nature and sends no
`Idempotency-Key`: canceling an already-canceled send answers the same
`200` envelope again (no second event). Once dispatched (or for an
immediate send's id) the server answers a 409 `not-cancellable`; an
unknown id is a 404.

Notes:

- `SendAt` with *inline* batch rows is rejected with a 422 `batch-invalid`
  (no batch cancel resource exists by design) — schedule rows individually
  via `Messages.Send`, or use the file form (rows expand and send at
  dispatch time; the file-form envelope answers `"scheduled"`).
- Scheduled creates stay always-keyed, exactly like immediate ones — an
  idempotent replay returns the scheduled envelope.
- On a scheduled broadcast envelope, `TargetCount` / `SkippedCount` may be
  null (decoded as `0`) until the audience resolves at dispatch time.
- Statuses: `scheduled` and `canceled` join the documented sets — message
  `scheduled|queued|sent|failed|canceled`, broadcast
  `scheduled|in_progress|completed|failed|canceled`.
- Test-mode (`sk_test_`) scheduled sends simulate on dispatch, like any
  other test-mode traffic.

## Suppressions

A per-workspace do-not-contact list, enforced on **every** send path. A
row matches on `(address, channel)` or — with no channel — on the address
across all channels; addresses are stored normalized (compact E.164 /
lowercase email), so any formatting of the same address matches.

```go
// Suppress an address on one channel (omit Channel for all channels)
sup, err := client.Suppressions.Create(ctx, silon.SuppressionCreateParams{
    Address: "+96550001234",
    Channel: silon.String("sms"),
    Reason:  silon.String(silon.SuppressionReasonStop), // default "manual"
})

// List (cursor-paginated) with optional filters
page, err := client.Suppressions.List(ctx, silon.SuppressionListParams{
    Reason: silon.String(silon.SuppressionReasonUnsubscribe),
})
for sup, err := range page.All(ctx) {
    if err != nil {
        return err
    }
    fmt.Println(sup.Address, sup.Reason) // sup.Channel == nil => all channels
}

// Make the address contactable again
err = client.Suppressions.Delete(ctx, sup.ID)
```

`Create` is idempotent by nature: creating a duplicate `(Address,
Channel)` in the same mode answers `200` with the *existing* suppression —
never an error — so it sends no `Idempotency-Key`. Reasons: `manual`,
`unsubscribe`, `hard_bounce`, `stop` (the `silon.SuppressionReason*`
constants). `List` requires the `suppressions:read` scope; `Create` /
`Delete` require `suppressions:write`.

Enforcement:

- **Single-recipient sends** (`Messages.Send` with `To`, `OTP.Send`) to a
  suppressed address are rejected with a 422 `recipient-suppressed`.
- **Fan-outs** (broadcasts, batch inline rows, batch file/CSV rows, legacy
  bulk) *skip* suppressed recipients instead — never an error. The
  broadcast/batch envelopes itemise the skips in the additive `Skipped`
  breakdown (`Suppressed` / `WrongChannel` / `Duplicate`), with
  `SkippedCount` staying the sum. Suppressed inline-batch rows are omitted
  from the per-row `Messages`; the file form reports its breakdown on the
  bulk read side (`Bulk.Retrieve`) once async expansion runs. `Skipped` is
  nil on servers predating the breakdown and on scheduled broadcast
  envelopes whose audience resolves at dispatch time.
- Suppressions are **mode-scoped**: test keys list/manage/enforce test
  suppressions only, live keys live ones.

Transactional/legal sends (e.g. a receipt owed to an unsubscribed
customer) can bypass a suppression per request — single-recipient sends
only:

```go
sent, err := client.Messages.Send(ctx, silon.MessageSendParams{
    Channel:             "email",
    To:                  map[string]any{"email": "sara@example.com"},
    Content:             map[string]any{"subject": "Your receipt", "body": "..."},
    OverrideSuppression: silon.Bool(true),
})
```

`OverrideSuppression` requires the `suppressions:override` scope, which is
in **no** scope preset and must be granted explicitly — without it the
request is a 403 `missing-scope`; alongside `Audience` (or on batches /
broadcasts) it is a 422 `override-not-allowed`. An overridden send
proceeds and its delivery row is flagged `suppression_overridden: true`.

## Templates

Slug-keyed message templates with an **immutable version spine** — the same
rows a send renders for `Template: {"slug": ...}`. Any change to a content
field (`Subject` / `Body` / `BodyMd`) mints an immutable version `N+1`;
`Channel` is metadata and never bumps the version.

```go
tmpl, err := client.Templates.Create(ctx, silon.TemplateCreateParams{
    Slug:    "order-shipped",
    Channel: silon.String("email"),
    Subject: silon.String("Your order shipped"),
    BodyMd:  silon.String("Hi {{ name }}, it's on the way."),
})
// tmpl.Version == 1, tmpl.Versions == []int{1}

updated, err := client.Templates.Update(ctx, "order-shipped", silon.TemplateUpdateParams{
    Subject: silon.String("Shipped today"),
}) // mints version 2

page, err := client.Templates.List(ctx, silon.TemplateListParams{Q: silon.String("order")})
err = client.Templates.Delete(ctx, "order-shipped") // soft archive
```

Pin an older revision on any send path (`Messages.Send`,
`Broadcasts.Create`, `Messages.SendBatch` row/defaults) with an optional
integer `version`; omit it to render the latest:

```go
sent, err := client.Messages.Send(ctx, silon.MessageSendParams{
    Channel:  "email",
    To:       map[string]any{"client_id": "cust_001"},
    Template: map[string]any{"slug": "order-shipped", "version": 1},
})
```

An unknown pinned version is a 422 `template-version-not-found`. `List` /
`Retrieve` require the `templates:read` scope; `Create` / `Update` /
`Delete` require `templates:write`. Delete is a soft archive: the slug
stays reserved (re-create is a 409 `template-exists`) and archived slugs
read as `template-not-found` everywhere.

## Webhook testing and delivery attempts

`WebhookEndpoints.Test` synchronously POSTs a signed `ping` to the endpoint
and returns the outcome — a failing sink is **not** an error (the result
carries `Delivered: false` and the reason in `Error`):

```go
result, err := client.WebhookEndpoints.Test(ctx, endpoint.ID)
if err != nil { /* auth / unknown id only */ }
fmt.Println(result.Delivered, result.ResponseStatus, result.LatencyMs, result.Error)
```

`WebhookEndpoints.ListAttempts` pages through the `(event, endpoint)`
delivery ledger (newest first) — each row is a `webhook_attempt` with
`Attempts`, `ResponseStatus` (nil when the endpoint never answered), `OK`,
`Error`, and the attempt timestamps. Test pings are never persisted and
never appear here.

## Test mode

Create an `sk_test_` API key (Settings → API keys) to integrate and CI-test
without a provider account or a real message. Test-mode requests run the
full pipeline — validation, scopes, rate limits, idempotency, delivery
rows, events — but never reach a provider and never bill. Affected
response models (`MessageAccepted`, `MessageStatus`, `BroadcastAccepted`,
`Broadcast`, `BatchAccepted`, `OTPSendResult`, `OTPVerifyResult`, `Event`,
`WebhookEndpoint`) carry a `Livemode` field: `false` for test traffic,
`true` for live.

Magic recipients force deterministic outcomes in test mode. Statuses
settle asynchronously a few seconds after the `202`, so polling and
webhooks behave realistically:

| Recipient | Outcome |
| --- | --- |
| `+15005550001` | delivered |
| `+15005550002` | failed (simulated provider error) |
| `+15005550009` | always suppressed (no suppression row needed) |
| `delivered@silon.test` | delivered |
| `bounce@silon.test` | failed |
| `suppressed@silon.test` | always suppressed (no suppression row needed) |
| anything else | delivered |

The always-suppressed recipients behave exactly like a real suppression:
a single send is rejected with a 422 `recipient-suppressed`, and a
fan-out skips them into the envelope's `Skipped.Suppressed` counter.

With a **live** key, magic recipients are rejected with a 422
`test-recipient-in-live` — test fixtures can never leak into real sends.

Test-mode OTPs are never dispatched; the magic code `000000` — and only
it — verifies:

```go
sent, err := client.OTP.Send(ctx, silon.OTPSendParams{
    Purpose: "login",
    To:      map[string]any{"phone_number": "+15005550001"},
})
result, err := client.OTP.Verify(ctx, silon.OTPVerifyParams{
    OTPID: sent.OTPID,
    Code:  "000000",
})
fmt.Println(result.Verified, result.Livemode) // true false
```

Webhook endpoints are mode-routed at create time: live endpoints receive
events from live sends only, and `Livemode: silon.Bool(false)` endpoints
receive test-mode events only — register one endpoint per mode to consume
both streams:

```go
endpoint, err := client.WebhookEndpoints.Create(ctx, silon.WebhookEndpointCreateParams{
    URL:      "https://ci.example.com/hooks/silon-test",
    Livemode: silon.Bool(false), // default is true (live events only)
})
```

## Resources

| Resource | Methods |
| --- | --- |
| `client.Messages` | `Send`, `SendBatch`, `Retrieve`, `Cancel` |
| `client.Broadcasts` | `Create`, `Retrieve`, `Deliveries` (paginated), `Cancel` |
| `client.OTP` | `Send`, `Verify` |
| `client.Clients` | `ListPage` (paginated), `List` (deprecated), `Create`, `Retrieve`, `Update`, `Replace`, `Delete` |
| `client.ClientGroups` | `ListPage` (paginated), `List` (deprecated), `Create`, `Retrieve`, `Update`, `Replace`, `Delete` |
| `client.Bulk` | `List`, `Retrieve`, `Send` (deprecated), `Files.List`, `Files.Upload`, `Recipients.Retrieve` |
| `client.Reports` | `Messages`, `Channels`, `Clients`, `Users`, `Bulks`, `SpecificBulks`, `Subscriptions`, `AWSUsage`, `Balance` |
| `client.WhatsAppTemplates` | `List`, `Retrieve` |
| `client.Templates` | `List` (paginated), `Create`, `Retrieve`, `Update`, `Delete` |
| `client.WebhookEndpoints` | `List` (paginated), `Create`, `Retrieve`, `Update`, `Delete`, `Test`, `ListAttempts` (paginated) |
| `client.Suppressions` | `List` (paginated), `Create`, `Delete` |
| `client.Events` | `List` (paginated), `Retrieve` |
| `client.Push` | `SubscribeAndroid`, `SubscribeIOS`, `UpsertDevices`, `MarkRead`, `ListNotifications`, `SubscribeWeb` |
| `client.Profile` | `Retrieve`, `Update`, `Replace` |
| `client.Auth` | `Signup` |

Deprecated operations (`Bulk.Send`, `Push.ListNotifications`,
`Push.SubscribeWeb`, `Clients.List`, `ClientGroups.List`)
carry Go's standard `// Deprecated:` doc-comment marker, which gopls and
staticcheck surface at the call site. `Bulk.Send`'s successor for every
shape is `Messages.SendBatch` (inline rows or a saved CSV via `File`);
`Bulk.Files.Upload` / `Files.List` stay current as the CSV ingestion path.

**CRM list grammar (C2).** The canonical CRM contacts and groups routes are
now plural and cursor-paginated (`/api/v1/crm/clients/`,
`/api/v1/crm/groups/`). `Clients.ListPage` / `ClientGroups.ListPage` are the
new paginated methods and return a `*silon.Page[T]`. The pre-C2
`Clients.List` / `ClientGroups.List` still work unchanged — they return a
bare `[]T` off the frozen singular routes and are safe for existing
`for _, x := range` / `len` / index call sites — but are now marked
deprecated in favour of `ListPage`. All CRM CRUD (`Create`, `Retrieve`,
`Update`, `Replace`, `Delete`) targets the canonical plural routes.

## Pagination

Cursor-paginated lists (`Events.List`, `WebhookEndpoints.List`,
`WebhookEndpoints.ListAttempts`, `Templates.List`, `Suppressions.List`,
`Broadcasts.Deliveries`, `Clients.ListPage`, `ClientGroups.ListPage`)
return a `*silon.Page[T]`
you can walk manually or drain with the lazy range-over-func iterator
`All`:

```go
page, err := client.Events.List(ctx, silon.EventListParams{
    Type:  silon.String("message.failed"),
    Limit: silon.Int(100),
})

for _, event := range page.Results { // this page only
    ...
}

for event, err := range page.All(ctx) { // every page, lazily
    if err != nil {
        return err
    }
    ...
}

// or manually
for page.HasNextPage() {
    page, err = page.NextPage(ctx)
    ...
}
```

`NextPage` on the last page (`HasNextPage() == false`) returns an error —
check `HasNextPage` first. `All` fetches each next page only when the
iteration reaches it and yields a single non-nil error (then stops) if a
page fetch fails.

## Errors

Non-2xx responses return a `*silon.APIError` carrying the parsed error
payload. Inspect it with `errors.As` or the status predicates:

```go
_, err := client.Messages.Send(ctx, silon.MessageSendParams{
    Channel: "banana",
    To:      map[string]any{"client_id": "x"},
})

var apiErr *silon.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.StatusCode, apiErr.RequestID)
    for _, detail := range apiErr.Errors {
        fmt.Println(detail.Code, detail.Attr, detail.Detail)
    }
}

if silon.IsRateLimit(err) { // 429
    fmt.Println("retry after", *apiErr.RetryAfter, "seconds")
}
```

Predicates: `IsBadRequest` (400), `IsAuthentication` (401),
`IsPermissionDenied` (403), `IsNotFound` (404), `IsConflict` (409,
idempotency-key reuse), `IsGone` (410, expired OTP), `IsUnprocessableEntity`
(422), `IsRateLimit` (429, with `RetryAfter` parsed from `Retry-After` /
`RateLimit-Reset`), `IsInternalServer` (5xx).

`APIError.Retryable` (`*bool`) mirrors the error body's `retryable` flag:
`true` iff retrying the *same* request could ever succeed (429, 5xx, or an
in-flight idempotency twin), `false` for every other 4xx. It is read
verbatim from the body — never recomputed from the status code — and is
`nil` when a legacy / non-v1 body omits the field.

Transport failures (the request never produced an HTTP response) are
`*silon.ConnectionError` — its `Timeout` field is true for timeouts and
`Unwrap()` exposes the underlying error. Configuration, client-side
validation, and response-parse failures are the base `*silon.Error`.

### Retries

Requests are retried automatically (default `WithMaxRetries(2)`, exponential
backoff with jitter, honouring `Retry-After` / `RateLimit-Reset`, delays
clamped to 30s) — but only when it is safe: idempotent methods
(GET/HEAD/OPTIONS/PUT/DELETE) plus POSTs that carry an `Idempotency-Key`,
and only on connection errors or HTTP 429/500/502/503/504. Other POST/PATCH
requests are never retried. The same `Idempotency-Key` value is replayed on
every attempt, and retry sleeps respect `context.Context` cancellation.

## Webhooks

Verify the `Silon-Signature` header on deliveries with the endpoint's
one-time `whsec_` secret — no HTTP client needed:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body) // raw bytes, not parsed JSON

    event, err := silon.ConstructWebhookEvent(
        body, r.Header.Get(silon.SignatureHeader),
        os.Getenv("SILON_WEBHOOK_SECRET"), silon.DefaultWebhookTolerance,
    )
    if err != nil {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    if event.Type == "broadcast.completed" {
        fmt.Println(*event.Data.Sent, "delivered,", *event.Data.Failed, "failed")
    }
}
```

`VerifyWebhookSignature(payload, header, secret, tolerance)` returns a bool
(constant-time compare; `tolerance <= 0` skips the freshness check;
malformed headers are false, never an error). `SignWebhookPayload` produces
valid headers for tests and mocks.

## Configuration

| Option | Env var | Default |
| --- | --- | --- |
| `WithAPIKey` | `SILON_API_KEY` | — (required) |
| `WithWorkspace` | `SILON_WORKSPACE` | — |
| `WithBaseURL` | `SILON_BASE_URL` | `https://<workspace>.silon.tech` |
| `WithTimeout` | — | 30 s |
| `WithMaxRetries` | — | 2 |
| `WithDefaultHeader` | — | — |
| `WithHTTPClient` | — | internally built `*http.Client` |

A base URL must be resolvable at construction time, from one of four
sources checked in this order — otherwise `NewClient` returns a
`*silon.Error` immediately (errors are returned, never panicked):

1. `WithBaseURL(...)` (wins over everything)
2. `SILON_BASE_URL` env var
3. `WithWorkspace(...)` → `https://<workspace>.silon.tech`
4. `SILON_WORKSPACE` env var → same expansion

A trailing slash on the base URL is stripped. `WithDefaultHeader(k, v)` adds
a header to every request (repeatable; later values for the same key win),
and `WithHTTPClient` supplies your own `*http.Client` for full transport
control.

## On-prem / self-hosted instances

The `WithWorkspace` shortcut is SaaS-only sugar; everything else in the SDK
is host-agnostic. For a self-hosted Silon, point `WithBaseURL` at your
instance:

```go
client, err := silon.NewClient(
    silon.WithAPIKey("sk_live_..."),
    silon.WithBaseURL("https://silon.customer.internal"),
)
```

API keys, the error contract, retries, idempotency, and webhook signature
verification all behave identically — they ride on the base URL.

**Private CA / self-signed TLS.** Supply your own `*http.Client` with a
custom root pool:

```go
caPEM, err := os.ReadFile("/etc/pki/customer-ca.pem")
if err != nil {
    log.Fatal(err)
}
pool := x509.NewCertPool()
pool.AppendCertsFromPEM(caPEM)

httpClient := &http.Client{
    Timeout: 30 * time.Second, // your client's Timeout governs
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{RootCAs: pool},
    },
}

client, err := silon.NewClient(
    silon.WithAPIKey("sk_live_..."),
    silon.WithBaseURL("https://10.20.0.5"),
    silon.WithHTTPClient(httpClient),
)
```

Note that a custom HTTP client brings its own timeout — set `Timeout` on the
`*http.Client`, since `WithTimeout` only applies to the transport the SDK
constructs itself.

**Reverse proxies.** Cursor pagination never follows the server's opaque
`next` URL directly — the SDK extracts only its query parameters and
re-issues the request against your configured base URL, so a proxy that
rewrites hostnames can't send pagination to an unreachable internal host.

## Platform adaptations (approved deviations from SPEC)

- **Single error type + predicates** instead of the per-status subclass
  hierarchy: `*APIError` with `Is*` helpers (Go has no exception subclassing).
- **Errors are returned, never panicked/thrown**; fail-fast construction
  means `NewClient` returns an error. Client-side validation failures (e.g.
  the `To`/`Audience` XOR on `Messages.Send`) return the base `*Error`
  where Python raises `ValueError`.
- **Unknown response fields are ignored** (Go `encoding/json` default)
  rather than preserved as dynamic attributes like Python's `extra="allow"`.
- **One synchronous client** — no separate async variant; use goroutines
  and `context.Context`.
- **`WithHTTPClient`** disables `WithTimeout`; the supplied client's own
  `Timeout` governs.
- **`Bulk.Files.Upload`** replaces Python's dynamically-typed file argument
  with `BulkFileUploadParams` — exactly one of `Path` / `Content` /
  `Reader` (a `*silon.Error` otherwise), with the multipart filename
  resolved as `Filename` override → `Path` base name → `Name()` of the
  reader (e.g. `*os.File`) → `recipients.csv`.
- **`BulkSendParams.Files`** matches the wire field name for message
  attachments, where Python names the keyword `attachments`.
- **Deprecated operations** (`Push.ListNotifications`, `Push.SubscribeWeb`)
  carry Go's standard `// Deprecated:` doc-comment marker
  (surfaced by godoc/gopls/staticcheck) instead of Python's runtime
  `DeprecationWarning`.
- **`Push.ListNotifications`** takes a typed `PushPlatform`
  (`PushPlatformAndroid` / `PushPlatformIOS` / `PushPlatformCombined`, the
  zero value) instead of Python's `"android"|"ios"|None`; an unknown
  platform returns the base `*Error` client-side without a request.
- **`PushNotification.Date` stays a string** — the legacy feed returns
  date-only values, not ISO-8601 date-times.
- **`Auth.Signup` returns `*UserProfile`** (the created profile) rather
  than a distinct `SignupResult` subclass as in Python.

## Development

```bash
cd sdk/go
go test ./... -count=1
go vet ./...
```

The test suite is fully offline (`net/http/httptest`); no real network
calls.
