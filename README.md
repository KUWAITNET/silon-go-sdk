# Silon Go SDK

Go client for the [Silon](https://silon.tech) messaging platform API — send
messages on any channel (WhatsApp, SMS, email, push, web push, voice), manage
CRM contacts and groups, run bulk campaigns, consume events, and verify
webhooks. Stdlib only — zero third-party dependencies.

## Installation

```bash
go get github.com/silon-tech/silon-go-sdk
```

Requires Go 1.24+. The module depends only on the Go standard library.

```go
import "github.com/silon-tech/silon-go-sdk"
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
duplicates are deduped and counted in `SkippedCount`):

```go
// Email broadcast to a client group
result, err := client.Broadcasts.Create(ctx, silon.BroadcastCreateParams{
    Channel:  "email",
    Audience: map[string]any{"type": "client_group", "slug": "vip"},
    Content:  map[string]any{"subject": "We saved you a seat", "body": "<h1>Hello</h1>"},
})
fmt.Println(result.TargetCount, result.SkippedCount)

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
`silon.Int`, `silon.Bool`, `silon.Float`. Fields this SDK version does not
model can be passed via `ExtraBody`, merged into the request body last.

## Resources

| Resource | Methods |
| --- | --- |
| `client.Messages` | `Send`, `SendBatch`, `Retrieve` |
| `client.Broadcasts` | `Create`, `Retrieve`, `Deliveries` (paginated) |
| `client.OTP` | `Send`, `Verify` |
| `client.Clients` | `List`, `Create`, `Retrieve`, `Update`, `Replace`, `Delete` |
| `client.ClientGroups` | `List`, `Create`, `Retrieve`, `Update`, `Replace`, `Delete` |
| `client.Bulk` | `List`, `Retrieve`, `Send` (deprecated), `Files.List`, `Files.Upload`, `Recipients.Retrieve` |
| `client.Reports` | `Messages`, `Channels`, `Clients`, `Users`, `Bulks`, `SpecificBulks`, `Subscriptions`, `AWSUsage`, `Balance` |
| `client.WhatsAppTemplates` | `List`, `Retrieve` |
| `client.WebhookEndpoints` | `List` (paginated), `Create`, `Retrieve`, `Update`, `Delete` |
| `client.Events` | `List` (paginated), `Retrieve` |
| `client.Push` | `SubscribeAndroid`, `SubscribeIOS`, `UpsertDevices`, `MarkRead`, `ListNotifications`, `SubscribeWeb` |
| `client.Profile` | `Retrieve`, `Update`, `Replace` |
| `client.Auth` | `Signup`, `Login` (deprecated) |

Deprecated operations (`Bulk.Send`, `Push.ListNotifications`,
`Push.SubscribeWeb`, `Auth.Login`) carry Go's standard `// Deprecated:`
doc-comment marker, which gopls and staticcheck surface at the call site.
`Bulk.Send`'s successor for every shape is `Messages.SendBatch`
(inline rows or a saved CSV via `File`); `Bulk.Files.Upload` / `Files.List`
stay current as the CSV ingestion path.

## Pagination

Cursor-paginated lists (`Events.List`, `WebhookEndpoints.List`,
`Broadcasts.Deliveries`) return a `*silon.Page[T]` you can walk manually or
drain with the lazy range-over-func iterator `All`:

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
- **Deprecated operations** (`Push.ListNotifications`, `Push.SubscribeWeb`,
  `Auth.Login`) carry Go's standard `// Deprecated:` doc-comment marker
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
