package silon

import (
	"reflect"
	"testing"
	"time"
)

var endpointJSON = map[string]any{
	"id":             "we_01J1ABC",
	"object":         "webhook_endpoint",
	"url":            "https://example.com/hooks/silon",
	"description":    "prod",
	"enabled_events": []any{"message.failed"},
	"livemode":       true,
	"status":         "enabled",
	"created_at":     "2026-07-01T00:00:00Z",
}

func endpointWith(overrides map[string]any) map[string]any {
	body := map[string]any{}
	for k, v := range endpointJSON {
		body[k] = v
	}
	for k, v := range overrides {
		body[k] = v
	}
	return body
}

func TestWebhookEndpointCreateReturnsOneTimeSecret(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, endpointWith(map[string]any{
		"secret": "whsec_3sXqf9b2e1Tn",
	}))))
	c := newTestClient(t, m)

	created, err := c.WebhookEndpoints.Create(t.Context(), WebhookEndpointCreateParams{
		URL:           "https://example.com/hooks/silon",
		Description:   String("prod"),
		EnabledEvents: []string{"message.failed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Secret != "whsec_3sXqf9b2e1Tn" {
		t.Errorf("Secret = %q", created.Secret)
	}
	if created.ID != "we_01J1ABC" || created.Status != "enabled" {
		t.Errorf("created = %+v", created)
	}
	if !created.Livemode {
		t.Error("Livemode = false, want true on a default (live) endpoint")
	}
	wantCreatedAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if created.CreatedAt == nil || !created.CreatedAt.Equal(wantCreatedAt) {
		t.Errorf("CreatedAt = %v", created.CreatedAt)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/webhook_endpoints" {
		t.Errorf("%s %s (path must have no trailing slash)", last.method, last.path)
	}
	want := map[string]any{
		"url":            "https://example.com/hooks/silon",
		"description":    "prod",
		"enabled_events": []any{"message.failed"},
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestWebhookEndpointCreateMinimalDefaultsToAllEvents(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, endpointWith(map[string]any{
		"enabled_events": []any{"*"},
		"secret":         "whsec_x",
	}))))
	c := newTestClient(t, m)

	created, err := c.WebhookEndpoints.Create(t.Context(), WebhookEndpointCreateParams{
		URL: "https://example.com/h",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.EnabledEvents) != 1 || created.EnabledEvents[0] != "*" {
		t.Errorf("EnabledEvents = %v", created.EnabledEvents)
	}
	// Omitted optional fields must be absent so the server can default
	// (livemode in particular defaults to true server-side).
	want := map[string]any{"url": "https://example.com/h"}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestWebhookEndpointCreateTestModeSerializesLivemodeFalse(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, endpointWith(map[string]any{
		"livemode": false,
		"secret":   "whsec_test1",
	}))))
	c := newTestClient(t, m)

	created, err := c.WebhookEndpoints.Create(t.Context(), WebhookEndpointCreateParams{
		URL:      "https://example.com/hooks/test",
		Livemode: Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Livemode {
		t.Error("Livemode = true, want false on a test-mode endpoint")
	}
	want := map[string]any{
		"url":      "https://example.com/hooks/test",
		"livemode": false,
	}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestWebhookEndpointCreateLivemodeTrueSentExplicitly(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, endpointWith(map[string]any{
		"secret": "whsec_live1",
	}))))
	c := newTestClient(t, m)

	created, err := c.WebhookEndpoints.Create(t.Context(), WebhookEndpointCreateParams{
		URL:      "https://example.com/hooks/live",
		Livemode: Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.Livemode {
		t.Error("Livemode = false, want true")
	}
	want := map[string]any{
		"url":      "https://example.com/hooks/live",
		"livemode": true,
	}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestWebhookEndpointListPaginated(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{endpointJSON}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	page, err := c.WebhookEndpoints.List(t.Context(), WebhookEndpointListParams{Limit: Int(10)})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/webhook_endpoints" {
		t.Errorf("%s %s (path must have no trailing slash)", last.method, last.path)
	}
	if last.query.Get("limit") != "10" {
		t.Errorf("query = %v", last.query)
	}
	if len(page.Results) != 1 {
		t.Fatalf("Results = %+v", page.Results)
	}
	endpoint := page.Results[0]
	if endpoint.ID != "we_01J1ABC" || endpoint.URL != "https://example.com/hooks/silon" {
		t.Errorf("endpoint = %+v", endpoint)
	}
	if len(endpoint.EnabledEvents) != 1 || endpoint.EnabledEvents[0] != "message.failed" {
		t.Errorf("EnabledEvents = %v", endpoint.EnabledEvents)
	}
	if page.HasNextPage() {
		t.Error("HasNextPage() = true on the last page")
	}
}

func TestWebhookEndpointRetrieveNoTrailingSlash(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, endpointJSON)))
	c := newTestClient(t, m)

	endpoint, err := c.WebhookEndpoints.Retrieve(t.Context(), "we_01J1ABC")
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/webhook_endpoints/we_01J1ABC" {
		t.Errorf("%s %s (path must have no trailing slash)", last.method, last.path)
	}
	if endpoint.Description != "prod" || endpoint.Object != "webhook_endpoint" {
		t.Errorf("endpoint = %+v", endpoint)
	}
	if !endpoint.Livemode {
		t.Error("Livemode = false, want true on a live endpoint")
	}
}

func TestWebhookEndpointDisable(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, endpointWith(map[string]any{
		"status": "disabled",
	}))))
	c := newTestClient(t, m)

	updated, err := c.WebhookEndpoints.Update(t.Context(), "we_01J1ABC",
		WebhookEndpointUpdateParams{Status: String("disabled")})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "disabled" {
		t.Errorf("Status = %q", updated.Status)
	}
	last := m.lastCall(t)
	if last.method != "PATCH" || last.path != "/api/v1/webhook_endpoints/we_01J1ABC" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"status": "disabled"}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v (only the set field may be sent)", got, want)
	}
}

func TestWebhookEndpointUpdateEnabledEvents(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, endpointJSON)))
	c := newTestClient(t, m)

	_, err := c.WebhookEndpoints.Update(t.Context(), "we_01J1ABC", WebhookEndpointUpdateParams{
		EnabledEvents: []string{"message.delivered", "broadcast.completed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"enabled_events": []any{"message.delivered", "broadcast.completed"},
	}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestWebhookEndpointDelete(t *testing.T) {
	m := newMockAPI(t, always(stub{status: 204}))
	c := newTestClient(t, m)

	if err := c.WebhookEndpoints.Delete(t.Context(), "we_01J1ABC"); err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "DELETE" || last.path != "/api/v1/webhook_endpoints/we_01J1ABC" {
		t.Errorf("%s %s (path must have no trailing slash)", last.method, last.path)
	}
}

func TestWebhookEndpointTestDelivered(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"delivered":       true,
		"response_status": 200,
		"latency_ms":      184,
		"error":           nil,
	})))
	c := newTestClient(t, m)

	result, err := c.WebhookEndpoints.Test(t.Context(), "we_01J1ABC")
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/webhook_endpoints/we_01J1ABC/test" {
		t.Errorf("%s %s (path must have no trailing slash)", last.method, last.path)
	}
	if len(last.body) != 0 {
		t.Errorf("Test sent a request body %q, want none", last.body)
	}
	if last.header.Get("Idempotency-Key") != "" {
		t.Error("Test sent an Idempotency-Key, want none")
	}
	if !result.Delivered || result.ResponseStatus == nil || *result.ResponseStatus != 200 {
		t.Errorf("result = %+v", result)
	}
	if result.LatencyMs != 184 || result.Error != nil {
		t.Errorf("result = %+v", result)
	}
}

func TestWebhookEndpointTestFailingSinkIsNotAnError(t *testing.T) {
	// A failing sink is 200 with delivered:false — never an HTTP error.
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"delivered":       false,
		"response_status": 500,
		"latency_ms":      92,
		"error":           "HTTP 500",
	})))
	c := newTestClient(t, m)

	result, err := c.WebhookEndpoints.Test(t.Context(), "we_01J1ABC")
	if err != nil {
		t.Fatalf("failing sink must not error: %v", err)
	}
	if result.Delivered {
		t.Error("Delivered = true, want false for a 500 sink")
	}
	if result.ResponseStatus == nil || *result.ResponseStatus != 500 {
		t.Errorf("ResponseStatus = %v, want 500", result.ResponseStatus)
	}
	if result.Error == nil || *result.Error != "HTTP 500" {
		t.Errorf("Error = %v, want \"HTTP 500\"", result.Error)
	}
}

func TestWebhookEndpointListAttempts(t *testing.T) {
	attempt := map[string]any{
		"id":              "wha_01J1ATT",
		"object":          "webhook_attempt",
		"event_id":        "evt_01J1EVT",
		"event_type":      "message.delivered",
		"attempts":        2,
		"response_status": nil,
		"ok":              false,
		"error":           "connection refused",
		"last_attempt_at": "2026-07-04T12:00:00Z",
		"next_attempt_at": "2026-07-04T12:05:00Z",
		"created":         "2026-07-04T11:59:00Z",
	}
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{attempt}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	page, err := c.WebhookEndpoints.ListAttempts(t.Context(), "we_01J1ABC",
		WebhookAttemptListParams{Limit: Int(20)})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/webhook_endpoints/we_01J1ABC/attempts" {
		t.Errorf("%s %s (path must have no trailing slash)", last.method, last.path)
	}
	if last.query.Get("limit") != "20" {
		t.Errorf("query = %v", last.query)
	}
	if len(page.Results) != 1 {
		t.Fatalf("Results = %+v", page.Results)
	}
	row := page.Results[0]
	if row.ID != "wha_01J1ATT" || row.Object != "webhook_attempt" {
		t.Errorf("attempt = %+v", row)
	}
	if row.EventID != "evt_01J1EVT" || row.EventType != "message.delivered" {
		t.Errorf("attempt = %+v", row)
	}
	if row.Attempts != 2 || row.OK {
		t.Errorf("attempt = %+v", row)
	}
	// response_status: null decodes as nil; error carries the reason.
	if row.ResponseStatus != nil {
		t.Errorf("ResponseStatus = %v, want nil (endpoint never answered)", *row.ResponseStatus)
	}
	if row.Error == nil || *row.Error != "connection refused" {
		t.Errorf("Error = %v", row.Error)
	}
	wantNext := time.Date(2026, 7, 4, 12, 5, 0, 0, time.UTC)
	if row.NextAttemptAt == nil || !row.NextAttemptAt.Equal(wantNext) {
		t.Errorf("NextAttemptAt = %v", row.NextAttemptAt)
	}
}
