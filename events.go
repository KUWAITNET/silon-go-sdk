package silon

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// eventsPath has NO trailing slash — the events endpoints are exact.
const eventsPath = "/api/v1/events"

// EventData is the "data" payload carried inside an event envelope.
//
// The shape varies by the envelope's Type — message.delivered /
// message.failed carry a settled-message snapshot, while
// broadcast.completed carries aggregate counts. Every field is therefore
// optional; branch on the parent envelope's Type.
type EventData struct {
	ID          *string    `json:"id,omitempty"`
	Object      *string    `json:"object,omitempty"`
	Channel     *string    `json:"channel,omitempty"`
	Recipient   *string    `json:"recipient,omitempty"`
	ClientID    *string    `json:"client_id,omitempty"`
	Status      *string    `json:"status,omitempty"`
	Error       *string    `json:"error,omitempty"`
	BroadcastID *string    `json:"broadcast_id,omitempty"`
	Provider    *string    `json:"provider,omitempty"`
	ExternalID  *string    `json:"external_id,omitempty"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`

	// broadcast.completed only.
	TargetCount *int `json:"target_count,omitempty"`
	Sent        *int `json:"sent,omitempty"`
	Failed      *int `json:"failed,omitempty"`
}

// Event is an event envelope — the exact JSON returned by the Events API
// and POSTed to subscribed webhook endpoints.
type Event struct {
	// ID is the opaque event id, "evt_" prefixed.
	ID string `json:"id"`

	Object string `json:"object,omitempty"`

	// Type is "message.delivered" / "message.failed" / "broadcast.completed".
	Type string `json:"type"`

	APIVersion string     `json:"api_version,omitempty"`
	Created    *time.Time `json:"created,omitempty"`
	Data       EventData  `json:"data,omitempty"`
}

// EventsService reads the event stream your webhook endpoints are fed
// from (newest first). Access it via Client.Events.
type EventsService struct {
	client *Client
}

// EventListParams filter and paginate EventsService.List. Nil fields are
// omitted from the query.
type EventListParams struct {
	// Type filters to one event type, e.g. "message.failed".
	Type *string

	// Cursor resumes listing from an opaque pagination cursor.
	Cursor *string

	// Limit caps the page size.
	Limit *int
}

func (p EventListParams) values() url.Values {
	q := url.Values{}
	if p.Type != nil {
		q.Set("type", *p.Type)
	}
	if p.Cursor != nil {
		q.Set("cursor", *p.Cursor)
	}
	if p.Limit != nil {
		q.Set("limit", strconv.Itoa(*p.Limit))
	}
	return q
}

// List pages through past events, newest first (GET /api/v1/events —
// no trailing slash; cursor-paginated).
func (s *EventsService) List(ctx context.Context, params EventListParams) (*Page[Event], error) {
	return fetchPage[Event](ctx, s.client, eventsPath, params.values())
}

// Retrieve fetches one event by id (GET /api/v1/events/{event_id} — no
// trailing slash).
func (s *EventsService) Retrieve(ctx context.Context, eventID string) (*Event, error) {
	var out Event
	if err := s.client.get(ctx, eventsPath+"/"+url.PathEscape(eventID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
