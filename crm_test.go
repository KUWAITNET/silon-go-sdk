package silon

import (
	"reflect"
	"testing"
)

var crmClientResponse = map[string]any{
	"client_id":        "cust_001",
	"first_name":       "Sara",
	"last_name":        "Ahmad",
	"email":            "sara@example.com",
	"phone_number":     "+96512345678",
	"civil_id":         nil,
	"notes":            "",
	"default_language": "en",
	"default_channel":  "whatsapp",
}

var crmGroupResponse = map[string]any{
	"id":        7,
	"name":      "VIP",
	"slug":      "vip",
	"is_active": true,
	"clients":   []any{crmClientResponse},
}

func TestClientsList(t *testing.T) {
	second := map[string]any{}
	for k, v := range crmClientResponse {
		second[k] = v
	}
	second["client_id"] = "cust_002"
	m := newMockAPI(t, always(jsonStub(200, []any{crmClientResponse, second})))
	c := newTestClient(t, m)

	// Back-compat lock: the pre-C2 List() call site must keep compiling
	// (returns a bare []ClientProfile — for/range, len, index all work) and
	// must keep hitting the FROZEN singular route that answers a bare array.
	clients, err := c.Clients.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 2 {
		t.Fatalf("len = %d, want 2", len(clients))
	}
	var ids []string
	for _, cl := range clients { // legacy iteration shape must still work
		ids = append(ids, cl.ClientID)
	}
	if clients[0].ClientID != "cust_001" || clients[1].ClientID != "cust_002" {
		t.Errorf("clients = %+v", clients)
	}
	if clients[0].CivilID != nil {
		t.Errorf("CivilID = %v, want nil", clients[0].CivilID)
	}

	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/crm/client/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}

func TestClientsCreateDropsOmittedFields(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, crmClientResponse)))
	c := newTestClient(t, m)

	created, err := c.Clients.Create(t.Context(), ClientCreateParams{
		ClientID:    "cust_001",
		FirstName:   String("Sara"),
		PhoneNumber: String("+96512345678"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ClientID != "cust_001" {
		t.Errorf("ClientID = %q", created.ClientID)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/crm/clients/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"client_id":    "cust_001",
		"first_name":   "Sara",
		"phone_number": "+96512345678",
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestClientsRetrieve(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, crmClientResponse)))
	c := newTestClient(t, m)

	got, err := c.Clients.Retrieve(t.Context(), "cust_001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "sara@example.com" {
		t.Errorf("Email = %q", got.Email)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/crm/clients/cust_001/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}

func TestClientsUpdateUsesPatchAndDropsOmitted(t *testing.T) {
	updated := map[string]any{}
	for k, v := range crmClientResponse {
		updated[k] = v
	}
	updated["notes"] = "vip"
	m := newMockAPI(t, always(jsonStub(200, updated)))
	c := newTestClient(t, m)

	got, err := c.Clients.Update(t.Context(), "cust_001", ClientUpdateParams{Notes: String("vip")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Notes != "vip" {
		t.Errorf("Notes = %q", got.Notes)
	}

	last := m.lastCall(t)
	if last.method != "PATCH" || last.path != "/api/v1/crm/clients/cust_001/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"notes": "vip"}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestClientsReplaceUsesPut(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, crmClientResponse)))
	c := newTestClient(t, m)

	_, err := c.Clients.Replace(t.Context(), "cust_001", ClientUpdateParams{
		FirstName:   String("Sara"),
		LastName:    String("Ahmad"),
		Email:       String("sara@example.com"),
		PhoneNumber: String("+96512345678"),
	})
	if err != nil {
		t.Fatal(err)
	}

	last := m.lastCall(t)
	if last.method != "PUT" || last.path != "/api/v1/crm/clients/cust_001/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"first_name":   "Sara",
		"last_name":    "Ahmad",
		"email":        "sara@example.com",
		"phone_number": "+96512345678",
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestClientsDelete(t *testing.T) {
	m := newMockAPI(t, always(rawStub(204, "", nil)))
	c := newTestClient(t, m)

	if err := c.Clients.Delete(t.Context(), "cust_001"); err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "DELETE" || last.path != "/api/v1/crm/clients/cust_001/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d", m.callCount())
	}
}

func TestClientGroupsList(t *testing.T) {
	// Back-compat lock: List() stays a bare []ClientGroup off the frozen
	// singular route.
	m := newMockAPI(t, always(jsonStub(200, []any{crmGroupResponse})))
	c := newTestClient(t, m)

	groups, err := c.ClientGroups.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("len = %d, want 1", len(groups))
	}
	g := groups[0]
	if g.ID != 7 || g.Slug != "vip" || !g.IsActive {
		t.Errorf("group = %+v", g)
	}
	if len(g.Clients) != 1 || g.Clients[0].ClientID != "cust_001" {
		t.Errorf("Clients = %+v", g.Clients)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/crm/group/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}

func TestClientGroupsCreateWithMembership(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, crmGroupResponse)))
	c := newTestClient(t, m)

	group, err := c.ClientGroups.Create(t.Context(), ClientGroupCreateParams{
		Name:      "VIP",
		Slug:      "vip",
		ClientIDs: []string{"cust_001", "cust_002"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if group.Slug != "vip" {
		t.Errorf("Slug = %q", group.Slug)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/crm/groups/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"name":       "VIP",
		"slug":       "vip",
		"client_ids": []any{"cust_001", "cust_002"},
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestClientGroupsCreateDropsOmittedFields(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, crmGroupResponse)))
	c := newTestClient(t, m)

	if _, err := c.ClientGroups.Create(t.Context(), ClientGroupCreateParams{
		Name: "VIP", Slug: "vip",
	}); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"name": "VIP", "slug": "vip"}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestClientGroupsRetrieve(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, crmGroupResponse)))
	c := newTestClient(t, m)

	group, err := c.ClientGroups.Retrieve(t.Context(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if group.Name != "VIP" {
		t.Errorf("Name = %q", group.Name)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/crm/groups/7/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}

func TestClientGroupsUpdateMembership(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, crmGroupResponse)))
	c := newTestClient(t, m)

	_, err := c.ClientGroups.Update(t.Context(), 7, ClientGroupUpdateParams{
		ClientIDs: []string{"cust_003"},
		IsActive:  Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "PATCH" || last.path != "/api/v1/crm/groups/7/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"client_ids": []any{"cust_003"},
		"is_active":  false,
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestClientGroupsReplaceAndDelete(t *testing.T) {
	m := newMockAPI(t, func(_ int, c call) stub {
		if c.method == "DELETE" {
			return rawStub(204, "", nil)
		}
		return jsonStub(200, crmGroupResponse)
	})
	c := newTestClient(t, m)

	// Replace with an explicit empty membership empties the group.
	_, err := c.ClientGroups.Replace(t.Context(), 7, ClientGroupReplaceParams{
		Name: "VIP", Slug: "vip", ClientIDs: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	put := m.call(0)
	if put.method != "PUT" || put.path != "/api/v1/crm/groups/7/" {
		t.Errorf("%s %s", put.method, put.path)
	}
	want := map[string]any{"name": "VIP", "slug": "vip", "client_ids": []any{}}
	if got := put.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}

	if err := c.ClientGroups.Delete(t.Context(), 7); err != nil {
		t.Fatal(err)
	}
	del := m.lastCall(t)
	if del.method != "DELETE" || del.path != "/api/v1/crm/groups/7/" {
		t.Errorf("%s %s", del.method, del.path)
	}
}

// --- C2 canonical plural cursor list (ListPage) -----------------------------

// TestClientsListPage exercises the new cursor-paginated ListPage against the
// canonical plural /api/v1/crm/clients/ route.
func TestClientsListPage(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results":  []any{crmClientResponse},
		"next":     nil,
		"previous": nil,
	})))
	c := newTestClient(t, m)

	page, err := c.Clients.ListPage(t.Context(), ClientListParams{Limit: Int(50)})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 1 || page.Results[0].ClientID != "cust_001" {
		t.Errorf("Results = %+v", page.Results)
	}
	if page.HasNextPage() {
		t.Error("HasNextPage() = true on a single page")
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/crm/clients/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if last.query.Get("limit") != "50" {
		t.Errorf("limit = %q, want 50", last.query.Get("limit"))
	}
}

// TestClientGroupsListPage exercises the new cursor-paginated ListPage against
// the canonical plural /api/v1/crm/groups/ route.
func TestClientGroupsListPage(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results":  []any{crmGroupResponse},
		"next":     nil,
		"previous": nil,
	})))
	c := newTestClient(t, m)

	page, err := c.ClientGroups.ListPage(t.Context(), ClientGroupListParams{Cursor: String("c0")})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 1 || page.Results[0].Slug != "vip" {
		t.Errorf("Results = %+v", page.Results)
	}
	if len(page.Results[0].Clients) != 1 || page.Results[0].Clients[0].ClientID != "cust_001" {
		t.Errorf("nested Clients = %+v", page.Results[0].Clients)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/crm/groups/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if last.query.Get("cursor") != "c0" {
		t.Errorf("cursor = %q, want c0", last.query.Get("cursor"))
	}
}

// clientsTwoPageResponder serves page 1 (no cursor) then page 2 (cursor=pg2).
// The advertised next URL points at a foreign host to prove it is never
// followed directly — the SDK must re-request the configured base URL.
func clientsTwoPageResponder(_ int, c call) stub {
	profile := func(id string) map[string]any {
		p := map[string]any{}
		for k, v := range crmClientResponse {
			p[k] = v
		}
		p["client_id"] = id
		return p
	}
	switch c.query.Get("cursor") {
	case "":
		return jsonStub(200, map[string]any{
			"results":  []any{profile("cust_001"), profile("cust_002")},
			"next":     "https://internal-proxy.local" + clientsPath + "?cursor=pg2&limit=2",
			"previous": nil,
		})
	case "pg2":
		return jsonStub(200, map[string]any{
			"results":  []any{profile("cust_003")},
			"next":     nil,
			"previous": "https://internal-proxy.local" + clientsPath + "?limit=2",
		})
	default:
		return jsonStub(404, map[string]any{})
	}
}

// TestClientsListPageWalksAllPages drains ListPage across two pages with the
// lazy auto-pager and checks proxy-safe cursor re-request.
func TestClientsListPageWalksAllPages(t *testing.T) {
	m := newMockAPI(t, clientsTwoPageResponder)
	c := newTestClient(t, m)

	page, err := c.Clients.ListPage(t.Context(), ClientListParams{Limit: Int(2)})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for cl, err := range page.All(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, cl.ClientID)
	}
	want := []string{"cust_001", "cust_002", "cust_003"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	if m.callCount() != 2 {
		t.Errorf("calls = %d, want 2", m.callCount())
	}
	// Page 2 must be fetched against the configured base URL (not the foreign
	// host in `next`), carrying the merged cursor + original limit.
	last := m.lastCall(t)
	if last.path != "/api/v1/crm/clients/" {
		t.Errorf("path = %q, want the configured base URL path", last.path)
	}
	if last.query.Get("cursor") != "pg2" || last.query.Get("limit") != "2" {
		t.Errorf("second request query = %v", last.query)
	}
}
