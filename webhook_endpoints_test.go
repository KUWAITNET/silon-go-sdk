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
