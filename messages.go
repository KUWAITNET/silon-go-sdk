package silon

import (
	"context"
	"net/url"
)

const (
	messagesPath      = "/api/v1/messages/"
	messagesBatchPath = messagesPath + "batch/"
)

// MessagesService sends messages on any channel (POST /api/v1/messages/)
// and looks up delivery status. Access it via Client.Messages.
type MessagesService struct {
	client *Client
}

// MessageSendParams are the parameters for MessagesService.Send.
//
// Exactly one of To (single recipient) or Audience (broadcast selector)
// is required. All other fields are optional; nil fields are omitted from
// the request JSON. Fields not covered here can be passed via ExtraBody,
// which is merged into the body last (overriding on key collision).
type MessageSendParams struct {
	// Channel is required: "sms", "whatsapp", "email", "push", "web_push", ...
	Channel string

	// To targets a single recipient, e.g. {"client_id": ...},
	// {"phone_number": ...}, {"email": ...} or {"device_token": ...}.
	To map[string]any

	// Audience targets a broadcast, e.g. {"type": "client_group",
	// "slug": ...} or {"type": "client_ids", "client_ids": [...]}.
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

func (p MessageSendParams) body() (map[string]any, error) {
	if (p.To == nil) == (p.Audience == nil) {
		return nil, &Error{
			Message: "Provide exactly one of 'to' (single recipient) or 'audience' (broadcast).",
		}
	}
	body := map[string]any{"channel": p.Channel}
	if p.To != nil {
		body["to"] = p.To
	}
	if p.Audience != nil {
		body["audience"] = p.Audience
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
	return body, nil
}

// MessageBatchParams are the parameters for MessagesService.SendBatch.
//
// Messages is required. All other fields are optional; nil fields are
// omitted from the request JSON. Fields not covered here can be passed via
// ExtraBody, which is merged into the body last (overriding on key
// collision).
type MessageBatchParams struct {
	// Messages is required: 1-500 free-form rows, each the same shape as
	// a MessageSendParams body minus Audience (rows are single-recipient
	// by definition — a row carrying "audience" fails the batch with a
	// per-index 422 pointing at POST /api/v1/broadcasts/). "to" is
	// required per row; a row's own "channel" overrides the top-level
	// default Channel (one of the two must yield a channel); the content
	// fields ("content", "template", "whatsapp_template", ...) are the
	// same as on a single send. Rows are sent verbatim.
	Messages []map[string]any

	// Channel is the optional top-level default channel applied to rows
	// that do not carry their own: "sms", "whatsapp", "email", "push",
	// "web_push", ...
	Channel *string

	// IdempotencyKey is sent as the Idempotency-Key header. When empty, a
	// UUIDv4 is generated — the header is ALWAYS sent, and the same value
	// is replayed on every retry attempt, so a retry can never double-send.
	IdempotencyKey string

	// ExtraBody is merged into the request body last — an escape hatch
	// for fields this SDK version does not model.
	ExtraBody map[string]any
}

func (p MessageBatchParams) body() map[string]any {
	body := map[string]any{"messages": p.Messages}
	if p.Channel != nil {
		body["channel"] = *p.Channel
	}
	for k, v := range p.ExtraBody {
		body[k] = v
	}
	return body
}

// BatchMessage is one per-row envelope inside BatchAccepted.
type BatchMessage struct {
	// ID is the row's message tracking id, individually pollable at
	// GET /api/v1/messages/{id}/ (MessagesService.Retrieve).
	ID string `json:"id"`

	// Object is "message".
	Object string `json:"object"`

	Channel string `json:"channel"`
	Status  string `json:"status"`
}

// BatchAccepted is the 202 envelope from POST /api/v1/messages/batch/.
type BatchAccepted struct {
	// ID is the batch id. It identifies the accepted request; batches
	// have no GET endpoint.
	ID string `json:"id"`

	// Object is "batch".
	Object string `json:"object"`

	// Messages holds the per-row envelopes, in request order.
	Messages []BatchMessage `json:"messages"`
}

// MessageAccepted is the 202 envelope from POST /api/v1/messages/.
type MessageAccepted struct {
	// ID is the tracking id for the message, or the broadcast id.
	ID string `json:"id"`

	// Object is "message" for a single recipient, "broadcast" for an
	// audience fan-out.
	Object string `json:"object"`

	Channel string `json:"channel"`
	Status  string `json:"status"`

	// TargetCount (broadcast only) is the number of recipients targeted.
	TargetCount *int `json:"target_count,omitempty"`

	// SkippedCount (broadcast only) is the number of recipients skipped
	// (unsubscribed / unreachable).
	SkippedCount *int `json:"skipped_count,omitempty"`
}

// MessageStatusItem is one recipient row inside a message-status batch.
type MessageStatusItem struct {
	ClientID    string `json:"client_id"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
	IsRead      bool   `json:"is_read"`
	ReadCount   int    `json:"read_count"`
}

// MessageStatus is the body of GET /api/v1/messages/{event_id}/.
type MessageStatus struct {
	EventID  string              `json:"event_id"`
	IsSent   bool                `json:"is_sent"`
	Messages []MessageStatusItem `json:"messages"`
}

// Send sends a message on any channel (POST /api/v1/messages/, 202).
//
// Exactly one of params.To or params.Audience is required — a client-side
// *Error is returned otherwise. An Idempotency-Key header is always sent
// (auto-generated UUIDv4 when params.IdempotencyKey is empty), which makes
// automatic retries safe.
func (s *MessagesService) Send(ctx context.Context, params MessageSendParams) (*MessageAccepted, error) {
	body, err := params.body()
	if err != nil {
		return nil, err
	}
	key := params.IdempotencyKey
	if key == "" {
		key = newUUID()
	}
	var out MessageAccepted
	if err := s.client.post(ctx, messagesPath, body,
		map[string]string{"Idempotency-Key": key}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendBatch sends up to 500 independent, personalised messages in one call
// (POST /api/v1/messages/batch/, 202). Use it when every recipient gets
// different content; for one content fanned out to an audience, use
// BroadcastsService.Create.
//
// Validation is all-or-nothing: the server validates every row through the
// same per-channel rules as Send before anything is queued, and any invalid
// row fails the whole batch with a 422 whose Attr carries a per-index path
// (e.g. "messages[3].to.phone_number"). An empty list is a 422 slug
// "batch-empty"; more than 500 rows is a 422 slug "batch-too-large".
//
// An Idempotency-Key header is always sent (auto-generated UUIDv4 when
// params.IdempotencyKey is empty), which makes automatic retries safe —
// a replay returns the same body including identical per-row ids.
// Requires the messages:send scope.
func (s *MessagesService) SendBatch(ctx context.Context, params MessageBatchParams) (*BatchAccepted, error) {
	key := params.IdempotencyKey
	if key == "" {
		key = newUUID()
	}
	var out BatchAccepted
	if err := s.client.post(ctx, messagesBatchPath, params.body(),
		map[string]string{"Idempotency-Key": key}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retrieve looks up a queued/sent message batch by its tracking id
// (GET /api/v1/messages/{event_id}/).
func (s *MessagesService) Retrieve(ctx context.Context, eventID string) (*MessageStatus, error) {
	var out MessageStatus
	if err := s.client.get(ctx, messagesPath+url.PathEscape(eventID)+"/", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
