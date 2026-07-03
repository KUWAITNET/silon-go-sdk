package silon

import (
	"reflect"
	"testing"
)

var whatsAppTemplateResponse = map[string]any{
	"id":          3,
	"name":        "order_confirmed",
	"language":    "en",
	"category":    "UTILITY",
	"status":      "APPROVED",
	"external_id": "1234567890",
	"waba":        map[string]any{"id": 1, "name": "Silon Test"},
	"preview":     "Hi {{1}}, order {{2}} is confirmed.",
	"mode":        "structured",
	"variables": []any{
		map[string]any{"key": "body_1", "label": "Customer name"},
		map[string]any{"key": "body_2", "label": "Order id"},
	},
}

func TestWhatsAppTemplatesListWithFilters(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"count":   1,
		"results": []any{whatsAppTemplateResponse},
	})))
	c := newTestClient(t, m)

	templates, err := c.WhatsAppTemplates.List(t.Context(), WhatsAppTemplateListParams{
		Name:     String("order"),
		Language: String("en"),
		WABAID:   Int(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if templates.Count != 1 || len(templates.Results) != 1 {
		t.Fatalf("templates = %+v", templates)
	}
	tpl := templates.Results[0]
	if tpl.ID != 3 || tpl.Name != "order_confirmed" || tpl.Category != "UTILITY" {
		t.Errorf("template = %+v", tpl)
	}
	if len(tpl.Variables) != 2 || tpl.Variables[0].Key != "body_1" ||
		tpl.Variables[0].Label != "Customer name" {
		t.Errorf("Variables = %+v", tpl.Variables)
	}
	if tpl.Components != nil {
		t.Errorf("Components = %v, want nil on list rows", tpl.Components)
	}

	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/whatsapp/templates/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if got := last.query.Get("name"); got != "order" {
		t.Errorf("name query = %q", got)
	}
	if got := last.query.Get("language"); got != "en" {
		t.Errorf("language query = %q", got)
	}
	if got := last.query.Get("waba_id"); got != "1" {
		t.Errorf("waba_id query = %q", got)
	}
}

func TestWhatsAppTemplatesListNoFilters(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"count":   1,
		"results": []any{whatsAppTemplateResponse},
	})))
	c := newTestClient(t, m)

	templates, err := c.WhatsAppTemplates.List(t.Context(), WhatsAppTemplateListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if templates.Results[0].Name != "order_confirmed" {
		t.Errorf("Name = %q", templates.Results[0].Name)
	}
	if got := m.lastCall(t).query; len(got) != 0 {
		t.Errorf("query = %v, want empty", got)
	}
}

func TestWhatsAppTemplatesRetrieveIncludesComponents(t *testing.T) {
	detail := map[string]any{}
	for k, v := range whatsAppTemplateResponse {
		detail[k] = v
	}
	detail["components"] = []any{map[string]any{"type": "BODY", "text": "Hi {{1}}"}}

	m := newMockAPI(t, always(jsonStub(200, detail)))
	c := newTestClient(t, m)

	tpl, err := c.WhatsAppTemplates.Retrieve(t.Context(), 3)
	if err != nil {
		t.Fatal(err)
	}
	wantComponents := []map[string]any{{"type": "BODY", "text": "Hi {{1}}"}}
	if !reflect.DeepEqual(tpl.Components, wantComponents) {
		t.Errorf("Components = %v, want %v", tpl.Components, wantComponents)
	}
	if tpl.WABA.ID == nil || *tpl.WABA.ID != 1 {
		t.Errorf("WABA.ID = %v", tpl.WABA.ID)
	}
	if tpl.WABA.Name == nil || *tpl.WABA.Name != "Silon Test" {
		t.Errorf("WABA.Name = %v", tpl.WABA.Name)
	}

	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/whatsapp/templates/3/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}
