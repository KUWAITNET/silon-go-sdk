package silon

import (
	"reflect"
	"testing"
	"time"
)

var templateDetailJSON = map[string]any{
	"slug":     "order-shipped",
	"object":   "template",
	"channel":  "email",
	"subject":  "Your order shipped",
	"version":  3,
	"created":  "2026-07-01T00:00:00Z",
	"updated":  "2026-07-04T12:00:00Z",
	"body":     "<p>On its way</p>",
	"body_md":  "On its way",
	"versions": []any{1, 2, 3},
}

var templateRowJSON = map[string]any{
	"slug":    "order-shipped",
	"object":  "template",
	"channel": "email",
	"subject": "Your order shipped",
	"version": 3,
	"created": "2026-07-01T00:00:00Z",
	"updated": "2026-07-04T12:00:00Z",
}

func TestTemplateListPaginated(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{templateRowJSON}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	page, err := c.Templates.List(t.Context(), TemplateListParams{
		Channel: String("email"),
		Q:       String("order"),
		Limit:   Int(50),
	})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/templates/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if last.query.Get("channel") != "email" || last.query.Get("q") != "order" || last.query.Get("limit") != "50" {
		t.Errorf("query = %v", last.query)
	}
	if len(page.Results) != 1 {
		t.Fatalf("Results = %+v", page.Results)
	}
	row := page.Results[0]
	if row.Slug != "order-shipped" || row.Object != "template" || row.Version != 3 {
		t.Errorf("row = %+v", row)
	}
	if row.Channel == nil || *row.Channel != "email" {
		t.Errorf("Channel = %v, want email", row.Channel)
	}
	// List rows omit the detail-only fields.
	if row.Body != "" || row.Versions != nil {
		t.Errorf("list row leaked detail fields: %+v", row)
	}
	if page.HasNextPage() {
		t.Error("HasNextPage() = true on the last page")
	}
}

func TestTemplateCreate(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, templateDetailWith(map[string]any{
		"version":  1,
		"versions": []any{1},
	}))))
	c := newTestClient(t, m)

	tmpl, err := c.Templates.Create(t.Context(), TemplateCreateParams{
		Slug:    "order-shipped",
		Channel: String("email"),
		Subject: String("Your order shipped"),
		Body:    String("<p>On its way</p>"),
		BodyMd:  String("On its way"),
	})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/templates/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"slug":    "order-shipped",
		"channel": "email",
		"subject": "Your order shipped",
		"body":    "<p>On its way</p>",
		"body_md": "On its way",
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	if tmpl.Version != 1 || len(tmpl.Versions) != 1 || tmpl.Versions[0] != 1 {
		t.Errorf("new template = %+v", tmpl)
	}
}

func TestTemplateCreateMinimalOmitsUnsetFields(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, templateDetailWith(map[string]any{
		"channel":  nil,
		"version":  1,
		"versions": []any{1},
	}))))
	c := newTestClient(t, m)

	tmpl, err := c.Templates.Create(t.Context(), TemplateCreateParams{Slug: "welcome"})
	if err != nil {
		t.Fatal(err)
	}
	// Only slug is sent; the server defaults the rest.
	want := map[string]any{"slug": "welcome"}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	// A cross-channel template decodes Channel as nil.
	if tmpl.Channel != nil {
		t.Errorf("Channel = %v, want nil for a cross-channel template", *tmpl.Channel)
	}
}

func TestTemplateRetrieveParsesVersionList(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, templateDetailJSON)))
	c := newTestClient(t, m)

	tmpl, err := c.Templates.Retrieve(t.Context(), "order-shipped")
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/templates/order-shipped/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if tmpl.Slug != "order-shipped" || tmpl.Object != "template" {
		t.Errorf("template = %+v", tmpl)
	}
	if tmpl.Version != 3 {
		t.Errorf("Version = %d, want 3 (latest)", tmpl.Version)
	}
	if !reflect.DeepEqual(tmpl.Versions, []int{1, 2, 3}) {
		t.Errorf("Versions = %v, want [1 2 3]", tmpl.Versions)
	}
	if tmpl.Body != "<p>On its way</p>" || tmpl.BodyMd != "On its way" {
		t.Errorf("body = %q / %q", tmpl.Body, tmpl.BodyMd)
	}
	wantCreated := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if tmpl.Created == nil || !tmpl.Created.Equal(wantCreated) {
		t.Errorf("Created = %v", tmpl.Created)
	}
}

func TestTemplateUpdateMintsVersion(t *testing.T) {
	// The content PATCH mints version 4.
	m := newMockAPI(t, always(jsonStub(200, templateDetailWith(map[string]any{
		"subject":  "Shipped today",
		"version":  4,
		"versions": []any{1, 2, 3, 4},
	}))))
	c := newTestClient(t, m)

	tmpl, err := c.Templates.Update(t.Context(), "order-shipped", TemplateUpdateParams{
		Subject: String("Shipped today"),
	})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "PATCH" || last.path != "/api/v1/templates/order-shipped/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	// Only the set field may be sent.
	want := map[string]any{"subject": "Shipped today"}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	if tmpl.Version != 4 || len(tmpl.Versions) != 4 {
		t.Errorf("updated template = %+v (want version 4)", tmpl)
	}
}

func TestTemplateDelete(t *testing.T) {
	m := newMockAPI(t, always(stub{status: 204}))
	c := newTestClient(t, m)

	if err := c.Templates.Delete(t.Context(), "order-shipped"); err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "DELETE" || last.path != "/api/v1/templates/order-shipped/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}

func templateDetailWith(overrides map[string]any) map[string]any {
	body := map[string]any{}
	for k, v := range templateDetailJSON {
		body[k] = v
	}
	for k, v := range overrides {
		body[k] = v
	}
	return body
}
