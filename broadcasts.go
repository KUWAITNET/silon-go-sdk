package silon

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

const broadcastsPath = "/api/v1/broadcasts/"

// BroadcastsService inspects audience fan-outs created by
// MessagesService.Send with an Audience target: aggregate delivery counts
// and per-recipient delivery rows. Access it via Client.Broadcasts.
type BroadcastsService struct {
	client *Client
}

// Broadcast is the body of GET /api/v1/broadcasts/{broadcast_id}/ —
// aggregate delivery counts for one broadcast.
type Broadcast struct {
	// ID is the broadcast id ("br_" prefixed).
	ID string `json:"id"`

	// Channel is the channel slug the broadcast was sent on.
	Channel string `json:"channel"`

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
