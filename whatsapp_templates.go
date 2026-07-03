package silon

import (
	"context"
	"net/url"
	"strconv"
)

const whatsAppTemplatesPath = "/api/v1/whatsapp/templates/"

// WhatsAppTemplateVariable is one variable a caller must supply when
// sending the template.
type WhatsAppTemplateVariable struct {
	// Key is the exact name to use in the send whatsapp_template
	// variables map (e.g. "body_1", "header_1", "otp_code").
	Key string `json:"key"`

	// Label is a human-readable label for the variable (e.g. "{{1}}").
	Label string `json:"label,omitempty"`
}

// WABA identifies the WhatsApp Business Account a template belongs to.
type WABA struct {
	ID   *int    `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

// WhatsAppTemplate is a single approved WhatsApp template.
//
// Components (the raw Meta-shaped component payload) is only present on
// the detail endpoint (WhatsAppTemplatesService.Retrieve); the list
// endpoint omits it.
type WhatsAppTemplate struct {
	// ID is the local template id (the path param on the detail
	// endpoint).
	ID int `json:"id"`

	// Name is the template name — pass it as whatsapp_template.name
	// when sending a Meta-Cloud template.
	Name string `json:"name"`

	// Language is the template language code, e.g. "en".
	Language string `json:"language,omitempty"`

	// Category is MARKETING / UTILITY / AUTHENTICATION.
	Category string `json:"category,omitempty"`

	// Status is the Meta-mirrored status (always APPROVED here).
	Status string `json:"status,omitempty"`

	// ExternalID is the Meta template ID (blank until submitted).
	ExternalID string `json:"external_id,omitempty"`

	// WABA is the WhatsApp Business Account the template belongs to.
	WABA WABA `json:"waba"`

	// Preview is the rendered preview text of the template body.
	Preview string `json:"preview,omitempty"`

	// Mode is "auth" or "structured".
	Mode string `json:"mode,omitempty"`

	// Variables are the variable slots this template expects.
	Variables []WhatsAppTemplateVariable `json:"variables,omitempty"`

	// Components is the raw Meta-shaped component payload — detail
	// endpoint only; nil on list rows.
	Components []map[string]any `json:"components,omitempty"`
}

// WhatsAppTemplateList is the envelope returned by
// GET /api/v1/whatsapp/templates/.
type WhatsAppTemplateList struct {
	// Count is the number of matching templates.
	Count int `json:"count"`

	// Results are the approved templates on this page.
	Results []WhatsAppTemplate `json:"results"`
}

// WhatsAppTemplateListParams filter WhatsAppTemplatesService.List. Nil
// fields are omitted from the query.
type WhatsAppTemplateListParams struct {
	// Name filters by template name.
	Name *string

	// Language filters by language code, e.g. "en".
	Language *string

	// WABAID filters to one WhatsApp Business Account.
	WABAID *int
}

func (p WhatsAppTemplateListParams) values() url.Values {
	q := url.Values{}
	if p.Name != nil {
		q.Set("name", *p.Name)
	}
	if p.Language != nil {
		q.Set("language", *p.Language)
	}
	if p.WABAID != nil {
		q.Set("waba_id", strconv.Itoa(*p.WABAID))
	}
	return q
}

// WhatsAppTemplatesService lists approved WhatsApp templates
// (/api/v1/whatsapp/templates/). Access it via Client.WhatsAppTemplates.
type WhatsAppTemplatesService struct {
	client *Client
}

// List fetches approved WhatsApp templates, optionally filtered by
// name / language / WABA (GET /api/v1/whatsapp/templates/).
func (s *WhatsAppTemplatesService) List(ctx context.Context, params WhatsAppTemplateListParams) (*WhatsAppTemplateList, error) {
	var out WhatsAppTemplateList
	if err := s.client.get(ctx, whatsAppTemplatesPath, params.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retrieve fetches one template with its components and variables
// (GET /api/v1/whatsapp/templates/{id}/).
func (s *WhatsAppTemplatesService) Retrieve(ctx context.Context, templateID int) (*WhatsAppTemplate, error) {
	var out WhatsAppTemplate
	if err := s.client.get(ctx, whatsAppTemplatesPath+strconv.Itoa(templateID)+"/", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
