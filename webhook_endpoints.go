package silon

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// webhookEndpointsPath has NO trailing slash — the webhook-endpoint
// endpoints are exact.
const webhookEndpointsPath = "/api/v1/webhook_endpoints"

// WebhookEndpointsService manages outbound webhook subscriptions
// (/api/v1/webhook_endpoints). Access it via Client.WebhookEndpoints.
type WebhookEndpointsService struct {
	client *Client
}

// WebhookEndpoint is a webhook endpoint as returned by the API.
//
// The signing secret is never present here — it is revealed once, in the
// create response (WebhookEndpointWithSecret).
type WebhookEndpoint struct {
	// ID is the opaque endpoint id, "we_" prefixed.
	ID string `json:"id"`

	// Object is always the string "webhook_endpoint".
	Object string `json:"object,omitempty"`

	// URL is the HTTPS URL that event envelopes are POSTed to.
	URL string `json:"url"`

	// Description is a free-text label for the endpoint.
	Description string `json:"description,omitempty"`

	// EnabledEvents are the event types delivered to this endpoint, or
	// ["*"] for all.
	EnabledEvents []string `json:"enabled_events,omitempty"`

	// Livemode is the endpoint's mode routing, fixed at create time.
	// true: receives events from live sends only; false: receives events
	// from test-mode (sk_test_) sends only.
	Livemode bool `json:"livemode"`

	// Status is "enabled" (receiving deliveries) or "disabled".
	Status string `json:"status,omitempty"`

	// CreatedAt is when the endpoint was created.
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// WebhookEndpointWithSecret is the create response — a WebhookEndpoint
// plus the one-time signing secret.
type WebhookEndpointWithSecret struct {
	WebhookEndpoint

	// Secret is the signing secret ("whsec_" prefix). Shown ONCE, only
	// here — store it now; it is never returned again. Use it to verify
	// the Silon-Signature header on every delivery.
	Secret string `json:"secret"`
}

// WebhookEndpointListParams are the optional cursor-pagination parameters
// for WebhookEndpointsService.List. Nil fields are omitted from the query.
type WebhookEndpointListParams struct {
	// Cursor resumes listing from an opaque pagination cursor.
	Cursor *string

	// Limit caps the page size.
	Limit *int
}

func (p WebhookEndpointListParams) values() url.Values {
	q := url.Values{}
	if p.Cursor != nil {
		q.Set("cursor", *p.Cursor)
	}
	if p.Limit != nil {
		q.Set("limit", strconv.Itoa(*p.Limit))
	}
	return q
}

// WebhookEndpointTestResult is the result of a synchronous test ping
// (WebhookEndpointsService.Test). A failing sink is NOT an HTTP error —
// the call returns a result with Delivered false and the reason in Error.
type WebhookEndpointTestResult struct {
	// Delivered is true when the endpoint answered with a 2xx status.
	Delivered bool `json:"delivered"`

	// ResponseStatus is the HTTP status the endpoint answered with; nil
	// when no response arrived (timeout, connection refused, DNS failure).
	ResponseStatus *int `json:"response_status,omitempty"`

	// LatencyMs is the round-trip time of the ping in milliseconds.
	LatencyMs int `json:"latency_ms"`

	// Error is nil on success; otherwise "HTTP <status>" for a non-2xx
	// answer, or the transport failure (e.g. a timeout message).
	Error *string `json:"error,omitempty"`
}

// WebhookAttempt is one (event, endpoint) delivery ledger row returned by
// WebhookEndpointsService.ListAttempts.
type WebhookAttempt struct {
	// ID is the opaque attempt id, "wha_" prefixed.
	ID string `json:"id"`

	// Object is always the string "webhook_attempt".
	Object string `json:"object,omitempty"`

	// EventID is the id of the delivered event ("evt_" prefixed) — fetch
	// it via EventsService.Retrieve.
	EventID string `json:"event_id"`

	// EventType is the type of the delivered event, e.g.
	// "message.delivered".
	EventType string `json:"event_type"`

	// Attempts is how many delivery attempts have been made so far.
	Attempts int `json:"attempts"`

	// ResponseStatus is the HTTP status of the most recent attempt; nil
	// when the endpoint never answered (timeout / connection failure).
	ResponseStatus *int `json:"response_status,omitempty"`

	// OK is true once an attempt got a 2xx answer (delivery succeeded).
	OK bool `json:"ok"`

	// Error is the failure reason of the most recent attempt ("HTTP
	// <status>" or a transport error); nil when the delivery succeeded.
	Error *string `json:"error,omitempty"`

	// LastAttemptAt is when the most recent attempt ran; nil before the
	// first attempt.
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`

	// NextAttemptAt is when the next retry is scheduled; nil when the
	// delivery is settled (succeeded, or retries exhausted).
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`

	// Created is when the delivery was first enqueued.
	Created *time.Time `json:"created,omitempty"`
}

// WebhookAttemptListParams are the optional cursor-pagination parameters
// for WebhookEndpointsService.ListAttempts. Nil fields are omitted from
// the query.
type WebhookAttemptListParams struct {
	// Cursor resumes listing from an opaque pagination cursor.
	Cursor *string

	// Limit caps the page size.
	Limit *int
}

func (p WebhookAttemptListParams) values() url.Values {
	q := url.Values{}
	if p.Cursor != nil {
		q.Set("cursor", *p.Cursor)
	}
	if p.Limit != nil {
		q.Set("limit", strconv.Itoa(*p.Limit))
	}
	return q
}

// WebhookEndpointCreateParams are the parameters for
// WebhookEndpointsService.Create. Nil fields are omitted from the JSON.
type WebhookEndpointCreateParams struct {
	// URL is required: the HTTPS URL signed event envelopes are POSTed to.
	URL string

	// Description is an optional free-text label.
	Description *string

	// EnabledEvents are the event types to deliver. Nil lets the server
	// default to ["*"] (everything).
	EnabledEvents []string

	// Livemode fixes the endpoint's mode routing at create time. Nil lets
	// the server default to true (live events only); silon.Bool(false)
	// subscribes the endpoint to test-mode (sk_test_) events only.
	Livemode *bool
}

// WebhookEndpointUpdateParams are the parameters for
// WebhookEndpointsService.Update. Only non-nil fields are sent.
type WebhookEndpointUpdateParams struct {
	// URL is the new HTTPS delivery URL.
	URL *string

	// Description is the new free-text label.
	Description *string

	// EnabledEvents replaces the delivered event types (["*"] for all).
	EnabledEvents []string

	// Status set to "disabled" pauses deliveries without deleting the
	// endpoint; "enabled" resumes them.
	Status *string
}

// List pages through webhook endpoints (GET /api/v1/webhook_endpoints —
// no trailing slash; cursor-paginated).
func (s *WebhookEndpointsService) List(ctx context.Context, params WebhookEndpointListParams) (*Page[WebhookEndpoint], error) {
	return fetchPage[WebhookEndpoint](ctx, s.client, webhookEndpointsPath, params.values())
}

// Create subscribes an HTTPS URL to events (POST /api/v1/webhook_endpoints).
//
// The response includes the one-time signing secret ("whsec_" prefix) —
// store it now, it is never returned again. Set params.Livemode to
// silon.Bool(false) to receive test-mode (sk_test_) events instead of
// live ones — the mode is fixed at create time.
func (s *WebhookEndpointsService) Create(ctx context.Context, params WebhookEndpointCreateParams) (*WebhookEndpointWithSecret, error) {
	body := map[string]any{"url": params.URL}
	if params.Description != nil {
		body["description"] = *params.Description
	}
	if params.EnabledEvents != nil {
		body["enabled_events"] = params.EnabledEvents
	}
	if params.Livemode != nil {
		body["livemode"] = *params.Livemode
	}
	var out WebhookEndpointWithSecret
	if err := s.client.post(ctx, webhookEndpointsPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retrieve fetches one webhook endpoint by id
// (GET /api/v1/webhook_endpoints/{endpoint_id} — no trailing slash).
func (s *WebhookEndpointsService) Retrieve(ctx context.Context, endpointID string) (*WebhookEndpoint, error) {
	var out WebhookEndpoint
	if err := s.client.get(ctx, webhookEndpointsPath+"/"+url.PathEscape(endpointID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update partially updates a webhook endpoint
// (PATCH /api/v1/webhook_endpoints/{endpoint_id} — no trailing slash).
// Set Status to "disabled" to pause deliveries.
func (s *WebhookEndpointsService) Update(ctx context.Context, endpointID string, params WebhookEndpointUpdateParams) (*WebhookEndpoint, error) {
	body := map[string]any{}
	if params.URL != nil {
		body["url"] = *params.URL
	}
	if params.Description != nil {
		body["description"] = *params.Description
	}
	if params.EnabledEvents != nil {
		body["enabled_events"] = params.EnabledEvents
	}
	if params.Status != nil {
		body["status"] = *params.Status
	}
	var out WebhookEndpoint
	if err := s.client.patch(ctx, webhookEndpointsPath+"/"+url.PathEscape(endpointID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a webhook endpoint
// (DELETE /api/v1/webhook_endpoints/{endpoint_id} — no trailing slash,
// 204 on success).
func (s *WebhookEndpointsService) Delete(ctx context.Context, endpointID string) error {
	return s.client.delete(ctx, webhookEndpointsPath+"/"+url.PathEscape(endpointID))
}

// Test synchronously POSTs a signed "ping" envelope to the endpoint URL
// and returns the delivery result (POST /api/v1/webhook_endpoints/
// {endpoint_id}/test — no trailing slash, no request body).
//
// A failing sink is NOT an HTTP error: the call succeeds with
// result.Delivered false and the reason in result.Error. The endpoint id
// must match the key's mode (a live key tests livemode-true endpoints, a
// test key livemode-false ones); a mode mismatch or unknown id is a 404
// slug "resource-not-found". Test pings are never persisted and never
// appear in ListAttempts. Requires the webhooks:write scope.
func (s *WebhookEndpointsService) Test(ctx context.Context, endpointID string) (*WebhookEndpointTestResult, error) {
	var out WebhookEndpointTestResult
	if err := s.client.post(ctx, webhookEndpointsPath+"/"+url.PathEscape(endpointID)+"/test", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAttempts pages through the (event, endpoint) delivery ledger for one
// endpoint, newest first (GET /api/v1/webhook_endpoints/{endpoint_id}/
// attempts — no trailing slash; cursor-paginated). An unknown endpoint id
// is a 404 slug "resource-not-found". Requires the webhooks:read scope.
func (s *WebhookEndpointsService) ListAttempts(ctx context.Context, endpointID string, params WebhookAttemptListParams) (*Page[WebhookAttempt], error) {
	path := webhookEndpointsPath + "/" + url.PathEscape(endpointID) + "/attempts"
	return fetchPage[WebhookAttempt](ctx, s.client, path, params.values())
}
