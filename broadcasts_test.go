package silon

import (
	"testing"
	"time"
)

var broadcastJSON = map[string]any{
	"id":           "br_01J1",
	"channel":      "email",
	"target_count": 100,
	"queued":       0,
	"sent":         97,
	"failed":       3,
	"started_at":   "2026-07-01T10:00:00Z",
	"completed_at": "2026-07-01T10:05:00Z",
	"status":       "completed",
}

const deliveriesPath = "/api/v1/broadcasts/br_01J1/deliveries/"

func deliveryJSON(n int, status string) map[string]any {
	return map[string]any{
		"id":        "d-" + string(rune('0'+n)),
		"client_id": "cust_00" + string(rune('0'+n)),
		"status":    status,
		"sent_at":   "2026-07-01T10:01:00Z",
		"error":     nil,
	}
}

// deliveriesResponder serves two pages of delivery rows; the advertised
// next URL points at a foreign host to prove it is never followed.
func deliveriesResponder(n int, c call) stub {
	switch c.query.Get("cursor") {
	case "":
		return jsonStub(200, map[string]any{
			"results":  []any{deliveryJSON(1, "sent"), deliveryJSON(2, "sent")},
			"next":     "https://internal-proxy.local" + deliveriesPath + "?cursor=pg2&limit=2",
			"previous": nil,
		})
	case "pg2":
		return jsonStub(200, map[string]any{
			"results": []any{map[string]any{
				"id":        "d-3",
				"client_id": "",
				"status":    "failed",
				"sent_at":   nil,
				"error":     "Mailbox unavailable",
			}},
			"next":     nil,
			"previous": "https://internal-proxy.local" + deliveriesPath + "?limit=2",
		})
	default:
		return jsonStub(404, map[string]any{})
	}
}

func TestBroadcastRetrieve(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, broadcastJSON)))
	c := newTestClient(t, m)

	broadcast, err := c.Broadcasts.Retrieve(t.Context(), "br_01J1")
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/broadcasts/br_01J1/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if broadcast.ID != "br_01J1" || broadcast.Channel != "email" || broadcast.Status != "completed" {
		t.Errorf("broadcast = %+v", broadcast)
	}
	if broadcast.TargetCount != 100 || broadcast.Sent != 97 || broadcast.Failed != 3 || broadcast.Queued != 0 {
		t.Errorf("counts = %+v", broadcast)
	}
	want := time.Date(2026, 7, 1, 10, 5, 0, 0, time.UTC)
	if broadcast.CompletedAt == nil || !broadcast.CompletedAt.Equal(want) {
		t.Errorf("CompletedAt = %v, want %v", broadcast.CompletedAt, want)
	}
}

func TestBroadcastInProgressHasNilCompletedAt(t *testing.T) {
	body := map[string]any{}
	for k, v := range broadcastJSON {
		body[k] = v
	}
	body["queued"] = 40
	body["completed_at"] = nil
	body["status"] = "in_progress"
	m := newMockAPI(t, always(jsonStub(200, body)))
	c := newTestClient(t, m)

	broadcast, err := c.Broadcasts.Retrieve(t.Context(), "br_01J1")
	if err != nil {
		t.Fatal(err)
	}
	if broadcast.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil", broadcast.CompletedAt)
	}
	if broadcast.Status != "in_progress" || broadcast.Queued != 40 {
		t.Errorf("broadcast = %+v", broadcast)
	}
}

func TestBroadcastDeliveriesParamsForwarded(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	_, err := c.Broadcasts.Deliveries(t.Context(), "br_01J1",
		BroadcastDeliveriesParams{Cursor: String("abc"), Limit: Int(50)})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.path != deliveriesPath {
		t.Errorf("path = %q, want %q", last.path, deliveriesPath)
	}
	if last.query.Get("cursor") != "abc" || last.query.Get("limit") != "50" {
		t.Errorf("query = %v", last.query)
	}
}

func TestBroadcastDeliveriesManualNextPageMergesParams(t *testing.T) {
	m := newMockAPI(t, deliveriesResponder)
	c := newTestClient(t, m)

	page, err := c.Broadcasts.Deliveries(t.Context(), "br_01J1",
		BroadcastDeliveriesParams{Limit: Int(2)})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 2 || !page.HasNextPage() {
		t.Fatalf("page = %+v", page)
	}
	if page.Results[0].ClientID != "cust_001" || page.Results[0].Status != "sent" {
		t.Errorf("Results[0] = %+v", page.Results[0])
	}

	page2, err := page.NextPage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Results) != 1 || page2.HasNextPage() {
		t.Fatalf("page2 = %+v", page2)
	}
	failed := page2.Results[0]
	if failed.Status != "failed" || failed.Error == nil || *failed.Error != "Mailbox unavailable" {
		t.Errorf("failed row = %+v", failed)
	}
	if failed.SentAt != nil {
		t.Errorf("SentAt = %v, want nil", failed.SentAt)
	}
	// The cursor from the opaque next URL merges over the original params,
	// and the request stays on the original path + configured base URL.
	second := m.lastCall(t)
	if second.path != deliveriesPath {
		t.Errorf("path = %q, want %q (foreign next host must not be followed)", second.path, deliveriesPath)
	}
	if second.query.Get("cursor") != "pg2" || second.query.Get("limit") != "2" {
		t.Errorf("second request query = %v", second.query)
	}
}

func TestBroadcastDeliveriesAll(t *testing.T) {
	m := newMockAPI(t, deliveriesResponder)
	c := newTestClient(t, m)

	page, err := c.Broadcasts.Deliveries(t.Context(), "br_01J1", BroadcastDeliveriesParams{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for delivery, err := range page.All(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, delivery.ID)
	}
	if len(ids) != 3 || ids[0] != "d-1" || ids[2] != "d-3" {
		t.Errorf("ids = %v", ids)
	}
	if m.callCount() != 2 {
		t.Errorf("calls = %d, want 2", m.callCount())
	}
}
