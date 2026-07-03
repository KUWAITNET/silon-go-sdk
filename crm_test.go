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

	clients, err := c.Clients.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 2 {
		t.Fatalf("len = %d, want 2", len(clients))
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
	if last.method != "POST" || last.path != "/api/v1/crm/client/" {
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
	if last.method != "GET" || last.path != "/api/v1/crm/client/cust_001/" {
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
	if last.method != "PATCH" || last.path != "/api/v1/crm/client/cust_001/" {
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
	if last.method != "PUT" || last.path != "/api/v1/crm/client/cust_001/" {
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
	if last.method != "DELETE" || last.path != "/api/v1/crm/client/cust_001/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d", m.callCount())
	}
}

func TestClientGroupsList(t *testing.T) {
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
	if last.method != "POST" || last.path != "/api/v1/crm/group/" {
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
	if last.method != "GET" || last.path != "/api/v1/crm/group/7/" {
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
	if last.method != "PATCH" || last.path != "/api/v1/crm/group/7/" {
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
	if put.method != "PUT" || put.path != "/api/v1/crm/group/7/" {
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
	if del.method != "DELETE" || del.path != "/api/v1/crm/group/7/" {
		t.Errorf("%s %s", del.method, del.path)
	}
}
