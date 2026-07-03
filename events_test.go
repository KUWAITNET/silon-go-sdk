package silon

import (
	"iter"
	"testing"
	"time"
)

var fullEventJSON = map[string]any{
	"id":          "evt_01J1",
	"object":      "event",
	"type":        "message.failed",
	"api_version": "2026-06-28",
	"created":     "2026-07-01T10:00:00Z",
	"data": map[string]any{
		"id":           "msg_1",
		"object":       "message",
		"channel":      "sms",
		"recipient":    "+1",
		"client_id":    "cust_001",
		"status":       "failed",
		"error":        "Unreachable",
		"broadcast_id": "br_01J1",
		"provider":     "twilio",
		"external_id":  nil,
		"sent_at":      nil,
		"created_at":   "2026-07-01T09:59:00Z",
	},
}

func TestEventsListNoTrailingSlashAndFilters(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{fullEventJSON}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	page, err := c.Events.List(t.Context(), EventListParams{
		Type:  String("message.failed"),
		Limit: Int(10),
	})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/events" {
		t.Errorf("%s %s (path must have no trailing slash)", last.method, last.path)
	}
	if last.query.Get("type") != "message.failed" || last.query.Get("limit") != "10" {
		t.Errorf("query = %v", last.query)
	}
	if len(page.Results) != 1 || page.Results[0].Type != "message.failed" {
		t.Errorf("Results = %+v", page.Results)
	}
}

func TestEventsRetrieveNoTrailingSlash(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, fullEventJSON)))
	c := newTestClient(t, m)

	event, err := c.Events.Retrieve(t.Context(), "evt_01J1")
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/events/evt_01J1" {
		t.Errorf("%s %s (path must have no trailing slash)", last.method, last.path)
	}
	if event.ID != "evt_01J1" || event.Type != "message.failed" || event.APIVersion != "2026-06-28" {
		t.Errorf("event = %+v", event)
	}
	wantCreated := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	if event.Created == nil || !event.Created.Equal(wantCreated) {
		t.Errorf("Created = %v", event.Created)
	}
	if event.Data.Error == nil || *event.Data.Error != "Unreachable" {
		t.Errorf("Data.Error = %v", event.Data.Error)
	}
	if event.Data.BroadcastID == nil || *event.Data.BroadcastID != "br_01J1" {
		t.Errorf("Data.BroadcastID = %v", event.Data.BroadcastID)
	}
	if event.Data.SentAt != nil || event.Data.ExternalID != nil {
		t.Errorf("null data fields must stay nil: %+v", event.Data)
	}
}

func TestEventsListManualNextPage(t *testing.T) {
	m := newMockAPI(t, twoPageResponder)
	c := newTestClient(t, m)

	page, err := c.Events.List(t.Context(), EventListParams{Limit: Int(2)})
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasNextPage() {
		t.Fatal("HasNextPage() = false")
	}
	page2, err := page.NextPage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Results) != 1 || page2.Results[0].ID != "evt_3" {
		t.Errorf("page2.Results = %+v", page2.Results)
	}
	// The cursor from the opaque next URL merges over the original params
	// and the request stays on the configured base URL (foreign host in
	// the next URL is never followed).
	second := m.lastCall(t)
	if second.path != eventsPath {
		t.Errorf("path = %q, want %q", second.path, eventsPath)
	}
	if second.query.Get("cursor") != "abc" || second.query.Get("limit") != "2" {
		t.Errorf("second request query = %v", second.query)
	}
}

func TestEventsListAutoPagingIsLazy(t *testing.T) {
	m := newMockAPI(t, twoPageResponder)
	c := newTestClient(t, m)

	page, err := c.Events.List(t.Context(), EventListParams{Limit: Int(2)})
	if err != nil {
		t.Fatal(err)
	}
	next, stop := iter.Pull2(page.All(t.Context()))
	defer stop()

	event, err, ok := next()
	if !ok || err != nil || event.ID != "evt_1" {
		t.Fatalf("first item: %v %v %v", event, err, ok)
	}
	next() // evt_2, still page 1
	if m.callCount() != 1 {
		t.Errorf("calls = %d, want 1 (page 2 must not be fetched until needed)", m.callCount())
	}
	event, err, ok = next() // triggers the page-2 fetch
	if !ok || err != nil || event.ID != "evt_3" {
		t.Fatalf("third item: %v %v %v", event, err, ok)
	}
	if m.callCount() != 2 {
		t.Errorf("calls = %d, want 2", m.callCount())
	}
}
