package silon

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// templatesPath HAS a trailing slash. The template endpoints also answer
// without it (trailing-slash-optional), but the SDK sends it for
// consistency with the rest of the API surface.
const templatesPath = "/api/v1/templates/"

// TemplatesService manages slug-keyed message templates with an IMMUTABLE
// version spine — the same rows a send renders for template: {"slug": ...}
// (see MessageSendParams.Template). Changing a content field (Subject /
// Body / BodyMd) mints an immutable version N+1; a send pins an older
// revision with template: {"slug": ..., "version": N}. Access it via
// Client.Templates.
type TemplatesService struct {
	client *Client
}

// Template is a stored message template. The list rows (List) carry the
// metadata fields; the detail endpoints (Create, Retrieve, Update) ADD
// Body, BodyMd and Versions.
type Template struct {
	// Slug is the unique template identifier — the value sends reference
	// via template: {"slug": ...}.
	Slug string `json:"slug"`

	// Object is always the string "template".
	Object string `json:"object,omitempty"`

	// Channel is an optional channel hint (metadata only — it never
	// restricts which channel may render the template); nil for a
	// cross-channel template.
	Channel *string `json:"channel,omitempty"`

	// Subject is the subject line (used by email; notification title on
	// push).
	Subject string `json:"subject"`

	// Version is the latest version number (immutable ints starting at 1).
	Version int `json:"version"`

	// Created is when the template was created.
	Created *time.Time `json:"created,omitempty"`

	// Updated is when the template last changed.
	Updated *time.Time `json:"updated,omitempty"`

	// Body is the latest HTML body — detail endpoints only (empty on list
	// rows).
	Body string `json:"body,omitempty"`

	// BodyMd is the latest markdown body — detail endpoints only (empty on
	// list rows).
	BodyMd string `json:"body_md,omitempty"`

	// Versions are all available version numbers, ascending — detail
	// endpoints only (nil on list rows). Pin one on a send via
	// template: {"slug": ..., "version": N}.
	Versions []int `json:"versions,omitempty"`
}

// TemplateListParams filter and paginate TemplatesService.List. Nil fields
// are omitted from the query.
type TemplateListParams struct {
	// Channel filters to templates carrying this channel hint (e.g. "sms").
	Channel *string

	// Q is a slug-prefix search — "order" matches "order-shipped",
	// "order-refund", ...
	Q *string

	// Cursor resumes listing from an opaque pagination cursor.
	Cursor *string

	// Limit caps the page size.
	Limit *int
}

func (p TemplateListParams) values() url.Values {
	q := url.Values{}
	if p.Channel != nil {
		q.Set("channel", *p.Channel)
	}
	if p.Q != nil {
		q.Set("q", *p.Q)
	}
	if p.Cursor != nil {
		q.Set("cursor", *p.Cursor)
	}
	if p.Limit != nil {
		q.Set("limit", strconv.Itoa(*p.Limit))
	}
	return q
}

// TemplateCreateParams are the parameters for TemplatesService.Create. Nil
// fields are omitted from the JSON.
type TemplateCreateParams struct {
	// Slug is required and unique: lowercase letters, digits, hyphens and
	// underscores. It is the value sends reference via template: {"slug":
	// ...}. A duplicate slug (including an archived one) is a 409 slug
	// "template-exists".
	Slug string

	// Channel is an optional channel hint (metadata only — never restricts
	// rendering). Nil omits it for a cross-channel template.
	Channel *string

	// Subject is the subject line (email / push title). Nil lets the
	// server default to "".
	Subject *string

	// Body is the HTML body. Nil lets the server default to "".
	Body *string

	// BodyMd is the markdown body. Nil lets the server default to "".
	BodyMd *string
}

// TemplateUpdateParams are the parameters for TemplatesService.Update. Only
// non-nil fields are sent; the server requires at least one. Changing any
// CONTENT field (Subject / Body / BodyMd) mints an immutable version N+1;
// Channel is metadata and never bumps the version, and a no-op content
// PATCH mints nothing.
type TemplateUpdateParams struct {
	// Channel is the new channel hint (metadata — no version bump).
	Channel *string

	// Subject is the new subject line (content — mints a new version when
	// changed).
	Subject *string

	// Body is the new HTML body (content — mints a new version when
	// changed).
	Body *string

	// BodyMd is the new markdown body (content — mints a new version when
	// changed).
	BodyMd *string
}

// List pages through the workspace's templates
// (GET /api/v1/templates/ — cursor-paginated), optionally filtered by
// Channel or a slug-prefix Q. Requires the templates:read scope.
func (s *TemplatesService) List(ctx context.Context, params TemplateListParams) (*Page[Template], error) {
	return fetchPage[Template](ctx, s.client, templatesPath, params.values())
}

// Create adds a template at version 1 (POST /api/v1/templates/, 201).
// A duplicate slug (including a previously archived one) is a 409 slug
// "template-exists". Requires the templates:write scope.
func (s *TemplatesService) Create(ctx context.Context, params TemplateCreateParams) (*Template, error) {
	body := map[string]any{"slug": params.Slug}
	if params.Channel != nil {
		body["channel"] = *params.Channel
	}
	if params.Subject != nil {
		body["subject"] = *params.Subject
	}
	if params.Body != nil {
		body["body"] = *params.Body
	}
	if params.BodyMd != nil {
		body["body_md"] = *params.BodyMd
	}
	var out Template
	if err := s.client.post(ctx, templatesPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retrieve fetches one template with its latest content and version list
// (GET /api/v1/templates/{slug}/). An unknown or archived slug is a 404
// slug "template-not-found". Requires the templates:read scope.
func (s *TemplatesService) Retrieve(ctx context.Context, slug string) (*Template, error) {
	var out Template
	if err := s.client.get(ctx, templatesPath+url.PathEscape(slug)+"/", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update partially updates a template (PATCH /api/v1/templates/{slug}/).
// The server requires at least one of Channel, Subject, Body or BodyMd;
// changing a content field mints a new immutable version. An unknown or
// archived slug is a 404 slug "template-not-found". Requires the
// templates:write scope.
func (s *TemplatesService) Update(ctx context.Context, slug string, params TemplateUpdateParams) (*Template, error) {
	body := map[string]any{}
	if params.Channel != nil {
		body["channel"] = *params.Channel
	}
	if params.Subject != nil {
		body["subject"] = *params.Subject
	}
	if params.Body != nil {
		body["body"] = *params.Body
	}
	if params.BodyMd != nil {
		body["body_md"] = *params.BodyMd
	}
	var out Template
	if err := s.client.patch(ctx, templatesPath+url.PathEscape(slug)+"/", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete soft-archives a template (DELETE /api/v1/templates/{slug}/, 204
// on success). Archived templates read as MISSING everywhere and sends
// referencing the slug fail resolution, but history and delivery-log FKs
// survive and the slug stays reserved (re-create is a 409 slug
// "template-exists"). An unknown or already-archived slug is a 404 slug
// "template-not-found". Requires the templates:write scope.
func (s *TemplatesService) Delete(ctx context.Context, slug string) error {
	return s.client.delete(ctx, templatesPath+url.PathEscape(slug)+"/")
}
