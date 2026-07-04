package silon

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// suppressionsPath HAS a trailing slash — the suppression endpoints are
// exact.
const suppressionsPath = "/api/v1/suppressions/"

// Suppression reasons accepted by SuppressionCreateParams.Reason and the
// SuppressionListParams.Reason filter.
const (
	SuppressionReasonManual      = "manual"
	SuppressionReasonUnsubscribe = "unsubscribe"
	SuppressionReasonHardBounce  = "hard_bounce"
	SuppressionReasonStop        = "stop"
)

// Suppression is one do-not-contact row as returned by the API.
type Suppression struct {
	// ID is the opaque suppression id, "sup_" prefixed.
	ID string `json:"id"`

	// Object is always the string "suppression".
	Object string `json:"object,omitempty"`

	// Address is the suppressed address, stored normalized (compact E.164
	// phone / lowercase email), so any formatting of the same address
	// matches.
	Address string `json:"address"`

	// Channel is the channel the suppression is scoped to (e.g. "sms");
	// nil means the address is suppressed on ALL channels.
	Channel *string `json:"channel,omitempty"`

	// Reason is why the address is suppressed: "manual", "unsubscribe",
	// "hard_bounce", or "stop".
	Reason string `json:"reason"`

	// Livemode is false when the row was created by an sk_test_ key —
	// test suppressions gate test sends only, live ones live sends.
	Livemode bool `json:"livemode"`

	// Created is when the suppression was created.
	Created *time.Time `json:"created,omitempty"`
}

// SuppressionsService manages the workspace's do-not-contact list
// (/api/v1/suppressions/). Rows are enforced mode-scoped on EVERY send
// path, matching on (address, channel) or (address, all channels):
// single-recipient sends (MessagesService.Send with To, OTPService.Send)
// to a suppressed address are rejected with a 422 slug
// "recipient-suppressed", while fan-outs (broadcasts, batches, bulk) skip
// suppressed recipients into the envelope's Skipped.Suppressed counter
// instead. Access it via Client.Suppressions.
type SuppressionsService struct {
	client *Client
}

// SuppressionListParams filter and paginate SuppressionsService.List. Nil
// fields are omitted from the query.
type SuppressionListParams struct {
	// Address filters to one address. It is normalized before matching,
	// so a formatted phone finds its compact row.
	Address *string

	// Channel filters to suppressions scoped to one channel (e.g. "sms").
	// All-channel rows (Channel == nil) are not matched by this filter.
	Channel *string

	// Reason filters by suppression reason — one of the
	// SuppressionReason* constants.
	Reason *string

	// Cursor resumes listing from an opaque pagination cursor.
	Cursor *string

	// Limit caps the page size.
	Limit *int
}

func (p SuppressionListParams) values() url.Values {
	q := url.Values{}
	if p.Address != nil {
		q.Set("address", *p.Address)
	}
	if p.Channel != nil {
		q.Set("channel", *p.Channel)
	}
	if p.Reason != nil {
		q.Set("reason", *p.Reason)
	}
	if p.Cursor != nil {
		q.Set("cursor", *p.Cursor)
	}
	if p.Limit != nil {
		q.Set("limit", strconv.Itoa(*p.Limit))
	}
	return q
}

// SuppressionCreateParams are the parameters for
// SuppressionsService.Create. Nil fields are omitted from the JSON.
type SuppressionCreateParams struct {
	// Address is required: an E.164 phone number ("+96550001234" —
	// separators tolerated) or an email address. Stored normalized
	// (compact E.164 / lowercase).
	Address string

	// Channel scopes the suppression to one channel (e.g. "sms",
	// "whatsapp", "email"). Nil suppresses the address on ALL channels.
	Channel *string

	// Reason is why the address is suppressed — one of the
	// SuppressionReason* constants. Nil lets the server default to
	// "manual".
	Reason *string
}

// List pages through the workspace's suppression list, newest first
// (GET /api/v1/suppressions/ — trailing slash; cursor-paginated),
// optionally filtered by Address, Channel, or Reason. Only rows in the
// key's mode are returned — test keys see test suppressions, live keys
// live ones. Requires the suppressions:read scope.
func (s *SuppressionsService) List(ctx context.Context, params SuppressionListParams) (*Page[Suppression], error) {
	return fetchPage[Suppression](ctx, s.client, suppressionsPath, params.values())
}

// Create adds an address to the do-not-contact list
// (POST /api/v1/suppressions/, 201). From that moment the address is
// enforced across every send surface in the key's mode.
//
// Create is IDEMPOTENT BY NATURE: creating a duplicate (Address, Channel)
// in the same mode answers 200 with the EXISTING suppression — never an
// error — so no Idempotency-Key header is sent. Requires the
// suppressions:write scope.
func (s *SuppressionsService) Create(ctx context.Context, params SuppressionCreateParams) (*Suppression, error) {
	body := map[string]any{"address": params.Address}
	if params.Channel != nil {
		body["channel"] = *params.Channel
	}
	if params.Reason != nil {
		body["reason"] = *params.Reason
	}
	var out Suppression
	if err := s.client.post(ctx, suppressionsPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a suppression, making the address contactable again
// (DELETE /api/v1/suppressions/{suppression_id}/ — trailing slash, 204 on
// success). An unknown or mode-mismatched id is a 404. Requires the
// suppressions:write scope.
func (s *SuppressionsService) Delete(ctx context.Context, suppressionID string) error {
	return s.client.delete(ctx, suppressionsPath+url.PathEscape(suppressionID)+"/")
}
