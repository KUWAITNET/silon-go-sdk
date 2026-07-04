package silon

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

const broadcastsPath = "/api/v1/broadcasts/"

// BroadcastsService creates broadcasts (one piece of content fanned out to
// an audience, POST /api/v1/broadcasts/) and inspects them: aggregate
// delivery counts and per-recipient delivery rows. Access it via
// Client.Broadcasts.
type BroadcastsService struct {
	client *Client
}

// BroadcastCreateParams are the parameters for BroadcastsService.Create.
//
// Channel and Audience are required. All other fields are optional; nil
// fields are omitted from the request JSON. Fields not covered here can be
// passed via ExtraBody, which is merged into the body last (overriding on
// key collision).
type BroadcastCreateParams struct {
	// Channel is required: "sms", "whatsapp", "email", "push", "web_push", ...
	Channel string

	// Audience is required and selects the recipients, e.g.
	// {"type": "client_group", "slug": ...},
	// {"type": "client_ids", "client_ids": [...]} or an inline ad-hoc list
	// {"type": "recipients", "recipients": [{"phone_number": ...},
	// {"email": ...}, {"client_id": ...}, ...]} (max 1,000 rows; duplicate
	// addresses are deduped and counted in SkippedCount).
	Audience map[string]any

	// Content is the message content, e.g. {"body": ...} and, for email,
	// {"subject": ...}.
	Content map[string]any

	// Template references a stored message template.
	Template map[string]any

	Provider    *string
	Sender      *string
	Application *string
	WidgetKey   *string
	Priority    *string
	TTL         *int

	// WhatsApp holds channel-specific options for WhatsApp sends.
	WhatsApp map[string]any

	// WhatsAppTemplate selects a WhatsApp template, e.g. {"name": ...,
	// "language": ..., "variables": {...}}.
	WhatsAppTemplate map[string]any

	// IdempotencyKey is sent as the Idempotency-Key header. When empty, a
	// UUIDv4 is generated — the header is ALWAYS sent, and the same value
	// is replayed on every retry attempt, so a retry can never double-send.
	IdempotencyKey string

	// ExtraBody is merged into the request body last — an escape hatch
	// for fields this SDK version does not model.
	ExtraBody map[string]any
}

func (p BroadcastCreateParams) body() map[string]any {
	body := map[string]any{
		"channel":  p.Channel,
		"audience": p.Audience,
	}
	if p.Content != nil {
		body["content"] = p.Content
	}
	if p.Template != nil {
		body["template"] = p.Template
	}
	if p.Provider != nil {
		body["provider"] = *p.Provider
	}
	if p.Sender != nil {
		body["sender"] = *p.Sender
	}
	if p.Application != nil {
		body["application"] = *p.Application
	}
	if p.WidgetKey != nil {
		body["widget_key"] = *p.WidgetKey
	}
	if p.Priority != nil {
		body["priority"] = *p.Priority
	}
	if p.TTL != nil {
		body["ttl"] = *p.TTL
	}
	if p.WhatsApp != nil {
		body["whatsapp"] = p.WhatsApp
	}
	if p.WhatsAppTemplate != nil {
		body["whatsapp_template"] = p.WhatsAppTemplate
	}
	for k, v := range p.ExtraBody {
		body[k] = v
	}
	return body
}

// BroadcastAccepted is the 202 envelope from POST /api/v1/broadcasts/.
type BroadcastAccepted struct {
	// ID is the broadcast id ("br_" prefixed).
	ID string `json:"id"`

	// Object is "broadcast".
	Object string `json:"object"`

	// Livemode is false when the broadcast ran in test mode (an sk_test_
	// key): nothing reaches a provider and nothing is billed.
	Livemode bool `json:"livemode"`

	Channel string `json:"channel"`
	Status  string `json:"status"`

	// TargetCount is the number of recipients targeted.
	TargetCount int `json:"target_count"`

	// SkippedCount is the number of recipients skipped (duplicates,
	// unsubscribed, unreachable).
	SkippedCount int `json:"skipped_count"`
}

// Broadcast is the body of GET /api/v1/broadcasts/{broadcast_id}/ —
// aggregate delivery counts for one broadcast.
type Broadcast struct {
	// ID is the broadcast id ("br_" prefixed).
	ID string `json:"id"`

	// Channel is the channel slug the broadcast was sent on.
	Channel string `json:"channel"`

	// Livemode is false when the broadcast ran in test mode (an sk_test_
	// key) — its per-recipient statuses are simulated. Nil when the
	// server does not report a mode.
	Livemode *bool `json:"livemode,omitempty"`

	// TargetCount is the total number of recipient rows in the broadcast.
	TargetCount int `json:"target_count"`

	// Queued is how many rows are still queued (not yet sent).
	Queued int `json:"queued"`

	// Sent is how many rows were successfully sent.
	Sent int `json:"sent"`

	// Failed is how many rows failed to send.
	Failed int `json:"failed"`

	// StartedAt is the timestamp of the earliest recipient row.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is the timestamp of the last send once nothing is left
	// queued; nil while the broadcast is still in progress.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Status is "completed" once nothing is queued, otherwise "in_progress".
	Status string `json:"status"`
}

// BroadcastDelivery is one per-recipient delivery row for a broadcast.
type BroadcastDelivery struct {
	// ID is the delivery's tracking id (UUID string).
	ID string `json:"id"`

	// ClientID is the external client identifier for this recipient (may
	// be blank).
	ClientID string `json:"client_id"`

	// Status is the delivery status, e.g. "pending", "queued", "sent",
	// "failed".
	Status string `json:"status"`

	// SentAt is when the row was sent; nil if not yet sent.
	SentAt *time.Time `json:"sent_at,omitempty"`

	// Error is the failure detail if the delivery failed; nil otherwise.
	Error *string `json:"error,omitempty"`
}

// BroadcastDeliveriesParams are the optional cursor-pagination parameters
// for BroadcastsService.Deliveries. Nil fields are omitted from the query.
type BroadcastDeliveriesParams struct {
	// Cursor resumes listing from an opaque pagination cursor.
	Cursor *string

	// Limit caps the page size.
	Limit *int
}

func (p BroadcastDeliveriesParams) values() url.Values {
	q := url.Values{}
	if p.Cursor != nil {
		q.Set("cursor", *p.Cursor)
	}
	if p.Limit != nil {
		q.Set("limit", strconv.Itoa(*p.Limit))
	}
	return q
}

// Create sends a broadcast — one piece of content fanned out to an
// audience — on any outbound channel (POST /api/v1/broadcasts/, 202).
//
// An Idempotency-Key header is always sent (auto-generated UUIDv4 when
// params.IdempotencyKey is empty), which makes automatic retries safe.
// Requires the broadcasts:send scope.
func (s *BroadcastsService) Create(ctx context.Context, params BroadcastCreateParams) (*BroadcastAccepted, error) {
	key := params.IdempotencyKey
	if key == "" {
		key = newUUID()
	}
	var out BroadcastAccepted
	if err := s.client.post(ctx, broadcastsPath, params.body(),
		map[string]string{"Idempotency-Key": key}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retrieve fetches aggregate delivery counts for a broadcast
// (GET /api/v1/broadcasts/{broadcast_id}/).
func (s *BroadcastsService) Retrieve(ctx context.Context, broadcastID string) (*Broadcast, error) {
	var out Broadcast
	if err := s.client.get(ctx, broadcastsPath+url.PathEscape(broadcastID)+"/", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Deliveries lists per-recipient delivery rows for a broadcast
// (GET /api/v1/broadcasts/{broadcast_id}/deliveries/, cursor-paginated).
func (s *BroadcastsService) Deliveries(ctx context.Context, broadcastID string, params BroadcastDeliveriesParams) (*Page[BroadcastDelivery], error) {
	path := broadcastsPath + url.PathEscape(broadcastID) + "/deliveries/"
	return fetchPage[BroadcastDelivery](ctx, s.client, path, params.values())
}
