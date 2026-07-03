package silon

import (
	"errors"
	"iter"
	"net/url"
	"strings"
	"testing"
)

func eventJSON(n int) map[string]any {
	return map[string]any{
		"id":          "evt_" + string(rune('0'+n)),
		"object":      "event",
		"type":        "message.delivered",
		"api_version": "2026-06-28",
		"created":     "2026-07-01T10:00:00Z",
		"data":        map[string]any{"id": "msg_1", "object": "message", "status": "sent"},
	}
}

// twoPageResponder serves page 1 (no cursor) then page 2 (cursor=abc). The
// advertised next URL deliberately points at a foreign host to prove it is
// never followed directly.
func twoPageResponder(n int, c call) stub {
	switch c.query.Get("cursor") {
	case "":
		return jsonStub(200, map[string]any{
			"results":  []any{eventJSON(1), eventJSON(2)},
			"next":     "https://internal-proxy.local" + eventsPath + "?cursor=abc&limit=2",
			"previous": nil,
		})
	case "abc":
		return jsonStub(200, map[string]any{
			"results":  []any{eventJSON(3)},
			"next":     nil,
			"previous": "https://internal-proxy.local" + eventsPath + "?limit=2",
		})
	default:
		return jsonStub(404, map[string]any{})
	}
}

func listEvents(t *testing.T, c *Client, params url.Values) *Page[Event] {
	t.Helper()
	page, err := fetchPage[Event](t.Context(), c, eventsPath, params)
	if err != nil {
		t.Fatalf("fetchPage: %v", err)
	}
	return page
}

func TestSinglePageIteration(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{eventJSON(1), eventJSON(2)}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	page := listEvents(t, c, nil)
	if len(page.Results) != 2 {
		t.Fatalf("len(Results) = %d", len(page.Results))
	}
	if page.Results[0].ID != "evt_1" || page.Results[1].ID != "evt_2" {
		t.Errorf("Results = %+v", page.Results)
	}
	if got := page.Results[0].Data.Status; got == nil || *got != "sent" {
		t.Errorf("Data.Status = %v", got)
	}
	if page.HasNextPage() {
		t.Error("HasNextPage() = true on the last page")
	}
	_, err := page.NextPage(t.Context())
	var baseErr *Error
	if !errors.As(err, &baseErr) || !strings.Contains(err.Error(), "no next page") {
		t.Errorf("NextPage on last page: err = %v", err)
	}
}

func TestManualNextPageMergesParams(t *testing.T) {
	m := newMockAPI(t, twoPageResponder)
	c := newTestClient(t, m)

	page := listEvents(t, c, url.Values{"limit": {"2"}})
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
	if page2.HasNextPage() {
		t.Error("page2.HasNextPage() = true")
	}
	// cursor from the opaque next URL is merged over our params.
	second := m.lastCall(t)
	if second.query.Get("cursor") != "abc" || second.query.Get("limit") != "2" {
		t.Errorf("second request query = %v", second.query)
	}
}

func TestAllWalksAllPages(t *testing.T) {
	m := newMockAPI(t, twoPageResponder)
	c := newTestClient(t, m)

	var ids []string
	for event, err := range listEvents(t, c, url.Values{"limit": {"2"}}).All(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, event.ID)
	}
	want := []string{"evt_1", "evt_2", "evt_3"}
	if len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	if m.callCount() != 2 {
		t.Errorf("calls = %d, want 2", m.callCount())
	}
}

func TestAllIsLazy(t *testing.T) {
	m := newMockAPI(t, twoPageResponder)
	c := newTestClient(t, m)

	page := listEvents(t, c, url.Values{"limit": {"2"}})
	next, stop := iter.Pull2(page.All(t.Context()))
	defer stop()

	event, err, ok := next()
	if !ok || err != nil || event.ID != "evt_1" {
		t.Fatalf("first item: %v %v %v", event, err, ok)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d after one item, want 1 (second page must not be fetched yet)", m.callCount())
	}
	next() // evt_2, still page 1
	if m.callCount() != 1 {
		t.Errorf("calls = %d, want 1", m.callCount())
	}
	event, err, ok = next() // triggers page 2 fetch
	if !ok || err != nil || event.ID != "evt_3" {
		t.Fatalf("third item: %v %v %v", event, err, ok)
	}
	if m.callCount() != 2 {
		t.Errorf("calls = %d, want 2", m.callCount())
	}
}

func TestForeignHostNextURLStaysOnBaseURL(t *testing.T) {
	// twoPageResponder's next URL points at internal-proxy.local; if the SDK
	// followed it directly, the httptest server would never see request 2.
	m := newMockAPI(t, twoPageResponder)
	c := newTestClient(t, m)

	var ids []string
	for event, err := range listEvents(t, c, nil).All(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, event.ID)
	}
	if len(ids) != 3 {
		t.Fatalf("ids = %v", ids)
	}
	last := m.lastCall(t)
	if last.path != eventsPath {
		t.Errorf("path = %q, want %q on the configured base URL", last.path, eventsPath)
	}
	if last.query.Get("cursor") != "abc" {
		t.Errorf("cursor = %q", last.query.Get("cursor"))
	}
}

func TestQueryParamsForwarded(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	listEvents(t, c, url.Values{"type": {"message.failed"}, "limit": {"10"}})
	q := m.lastCall(t).query
	if q.Get("type") != "message.failed" || q.Get("limit") != "10" {
		t.Errorf("query = %v", q)
	}
}
