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
// store it now, it is never returned again.
func (s *WebhookEndpointsService) Create(ctx context.Context, params WebhookEndpointCreateParams) (*WebhookEndpointWithSecret, error) {
	body := map[string]any{"url": params.URL}
	if params.Description != nil {
		body["description"] = *params.Description
	}
	if params.EnabledEvents != nil {
		body["enabled_events"] = params.EnabledEvents
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
