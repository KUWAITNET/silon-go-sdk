package silon

import (
	"context"
	"net/url"
	"strconv"
)

const (
	// clientsPath is the CANONICAL plural CRM contacts route
	// (/api/v1/crm/clients/) — cursor-paginated list plus CRUD.
	clientsPath = "/api/v1/crm/clients/"
	// clientsLegacyPath is the DEPRECATED singular route
	// (/api/v1/crm/client/). Its list still returns a frozen bare JSON
	// array; kept only so ClientsService.List stays source- and
	// behavior-compatible.
	clientsLegacyPath = "/api/v1/crm/client/"

	// clientGroupsPath is the CANONICAL plural CRM groups route
	// (/api/v1/crm/groups/) — cursor-paginated list plus CRUD.
	clientGroupsPath = "/api/v1/crm/groups/"
	// clientGroupsLegacyPath is the DEPRECATED singular route
	// (/api/v1/crm/group/) whose list returns a frozen bare JSON array.
	clientGroupsLegacyPath = "/api/v1/crm/group/"
)

// ClientProfile is a CRM contact (/api/v1/crm/clients/).
type ClientProfile struct {
	// ClientID is your stable identifier for this contact — the value
	// passed as to.client_id (or inside an audience) when sending a
	// message. Set once; immutable on update.
	ClientID string `json:"client_id"`

	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`

	// CivilID is the Kuwait Civil ID; nullable.
	CivilID *string `json:"civil_id,omitempty"`

	// Notes are free-text internal notes, never sent to the contact.
	Notes string `json:"notes,omitempty"`

	// DefaultLanguage is the two-letter code used to localise templated
	// sends ("en" / "ar").
	DefaultLanguage string `json:"default_language,omitempty"`

	// DefaultChannel is the preferred outbound channel for this contact
	// (e.g. "sms", "whatsapp", "email").
	DefaultChannel string `json:"default_channel,omitempty"`
}

// ClientGroup is a CRM client group (/api/v1/crm/groups/).
type ClientGroup struct {
	// ID is the server-assigned numeric id of the group.
	ID int `json:"id"`

	Name string `json:"name"`

	// Slug is the URL-safe identifier — pass it as audience.slug with
	// audience.type "client_group" to broadcast to this group.
	Slug string `json:"slug"`

	// IsActive is false for groups kept but excluded from broadcast
	// targeting.
	IsActive bool `json:"is_active"`

	// Clients holds the full member profiles (read-only). To change
	// membership, write ClientIDs on create/update/replace.
	Clients []ClientProfile `json:"clients,omitempty"`
}

// ClientsService manages CRM client profiles (/api/v1/crm/clients/).
// Access it via Client.Clients.
type ClientsService struct {
	client *Client
}

// clientProfileFields are the writable profile fields shared by create,
// update, and replace. Nil fields are omitted from the request JSON.
type clientProfileFields struct {
	FirstName       *string
	LastName        *string
	Email           *string
	PhoneNumber     *string
	CivilID         *string
	Notes           *string
	DefaultLanguage *string
	DefaultChannel  *string
}

func (f clientProfileFields) body() map[string]any {
	body := map[string]any{}
	if f.FirstName != nil {
		body["first_name"] = *f.FirstName
	}
	if f.LastName != nil {
		body["last_name"] = *f.LastName
	}
	if f.Email != nil {
		body["email"] = *f.Email
	}
	if f.PhoneNumber != nil {
		body["phone_number"] = *f.PhoneNumber
	}
	if f.CivilID != nil {
		body["civil_id"] = *f.CivilID
	}
	if f.Notes != nil {
		body["notes"] = *f.Notes
	}
	if f.DefaultLanguage != nil {
		body["default_language"] = *f.DefaultLanguage
	}
	if f.DefaultChannel != nil {
		body["default_channel"] = *f.DefaultChannel
	}
	return body
}

// ClientListParams paginate ClientsService.ListPage. Nil fields are
// omitted from the query.
type ClientListParams struct {
	// Cursor resumes listing from an opaque pagination cursor.
	Cursor *string

	// Limit caps the page size (default 50, max 100).
	Limit *int
}

func (p ClientListParams) values() url.Values {
	q := url.Values{}
	if p.Cursor != nil {
		q.Set("cursor", *p.Cursor)
	}
	if p.Limit != nil {
		q.Set("limit", strconv.Itoa(*p.Limit))
	}
	return q
}

// ClientCreateParams are the parameters for ClientsService.Create. Only
// ClientID is required; nil fields are omitted from the request JSON.
type ClientCreateParams struct {
	// ClientID is required: your stable identifier for the contact.
	ClientID string

	FirstName       *string
	LastName        *string
	Email           *string
	PhoneNumber     *string
	CivilID         *string
	Notes           *string
	DefaultLanguage *string
	DefaultChannel  *string
}

// ClientUpdateParams are the parameters for ClientsService.Update (PATCH,
// only non-nil fields are sent) and ClientsService.Replace (PUT). The
// client_id itself is immutable and set via the method argument.
type ClientUpdateParams struct {
	FirstName       *string
	LastName        *string
	Email           *string
	PhoneNumber     *string
	CivilID         *string
	Notes           *string
	DefaultLanguage *string
	DefaultChannel  *string
}

func (p ClientUpdateParams) body() map[string]any {
	return clientProfileFields(p).body()
}

// List fetches every CRM client in one request (GET /api/v1/crm/client/ —
// the deprecated singular route, which returns a bare JSON array rather
// than a paginated envelope).
//
// Deprecated: the singular list route is frozen for back-compat. Use
// ListPage, which targets the canonical cursor-paginated /api/v1/crm/clients/
// route and returns a *Page[ClientProfile] you can walk (or drain lazily
// with Page.All). This List remains source- and behavior-compatible and is
// not going away, but new code should prefer ListPage.
func (s *ClientsService) List(ctx context.Context) ([]ClientProfile, error) {
	var out []ClientProfile
	if err := s.client.get(ctx, clientsLegacyPath, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListPage returns one cursor-paginated page of CRM clients, newest first
// by registration_date (GET /api/v1/crm/clients/ — the canonical plural
// route). Walk further pages with Page.NextPage, or drain them all lazily
// with Page.All.
func (s *ClientsService) ListPage(ctx context.Context, params ClientListParams) (*Page[ClientProfile], error) {
	return fetchPage[ClientProfile](ctx, s.client, clientsPath, params.values())
}

// Create adds a CRM client (POST /api/v1/crm/clients/).
func (s *ClientsService) Create(ctx context.Context, params ClientCreateParams) (*ClientProfile, error) {
	body := clientProfileFields{
		FirstName:       params.FirstName,
		LastName:        params.LastName,
		Email:           params.Email,
		PhoneNumber:     params.PhoneNumber,
		CivilID:         params.CivilID,
		Notes:           params.Notes,
		DefaultLanguage: params.DefaultLanguage,
		DefaultChannel:  params.DefaultChannel,
	}.body()
	body["client_id"] = params.ClientID
	var out ClientProfile
	if err := s.client.post(ctx, clientsPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retrieve fetches one CRM client by its client_id
// (GET /api/v1/crm/clients/{client_id}/).
func (s *ClientsService) Retrieve(ctx context.Context, clientID string) (*ClientProfile, error) {
	var out ClientProfile
	if err := s.client.get(ctx, clientsPath+url.PathEscape(clientID)+"/", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update partially updates a CRM client
// (PATCH /api/v1/crm/clients/{client_id}/) — only non-nil fields change.
func (s *ClientsService) Update(ctx context.Context, clientID string, params ClientUpdateParams) (*ClientProfile, error) {
	var out ClientProfile
	if err := s.client.patch(ctx, clientsPath+url.PathEscape(clientID)+"/", params.body(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Replace fully replaces a CRM client
// (PUT /api/v1/crm/clients/{client_id}/). The client_id itself is
// immutable; omitted fields are reset server-side.
func (s *ClientsService) Replace(ctx context.Context, clientID string, params ClientUpdateParams) (*ClientProfile, error) {
	var out ClientProfile
	if err := s.client.put(ctx, clientsPath+url.PathEscape(clientID)+"/", params.body(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a CRM client (DELETE /api/v1/crm/clients/{client_id}/,
// 204 on success).
func (s *ClientsService) Delete(ctx context.Context, clientID string) error {
	return s.client.delete(ctx, clientsPath+url.PathEscape(clientID)+"/")
}

// ClientGroupsService manages CRM client groups (/api/v1/crm/groups/).
// Access it via Client.ClientGroups.
type ClientGroupsService struct {
	client *Client
}

// ClientGroupListParams paginate ClientGroupsService.ListPage. Nil fields
// are omitted from the query.
type ClientGroupListParams struct {
	// Cursor resumes listing from an opaque pagination cursor.
	Cursor *string

	// Limit caps the page size (default 50, max 100).
	Limit *int
}

func (p ClientGroupListParams) values() url.Values {
	q := url.Values{}
	if p.Cursor != nil {
		q.Set("cursor", *p.Cursor)
	}
	if p.Limit != nil {
		q.Set("limit", strconv.Itoa(*p.Limit))
	}
	return q
}

// ClientGroupCreateParams are the parameters for
// ClientGroupsService.Create. Name and Slug are required; nil fields are
// omitted from the request JSON.
type ClientGroupCreateParams struct {
	// Name is the human-readable group name.
	Name string

	// Slug is the URL-safe identifier used as audience.slug when
	// broadcasting to the group.
	Slug string

	// ClientIDs is the write-only membership list: the client_ids that
	// make up the group.
	ClientIDs []string

	// IsActive set to false excludes the group from broadcast targeting.
	IsActive *bool
}

// ClientGroupUpdateParams are the parameters for
// ClientGroupsService.Update (PATCH). Only non-nil fields are sent;
// ClientIDs, when non-nil, REPLACES the whole membership.
type ClientGroupUpdateParams struct {
	Name *string
	Slug *string

	// ClientIDs replaces the group membership when non-nil (write-only).
	ClientIDs []string

	IsActive *bool
}

// ClientGroupReplaceParams are the parameters for
// ClientGroupsService.Replace (PUT) — the full new state of the group.
type ClientGroupReplaceParams struct {
	Name string
	Slug string

	// ClientIDs is the complete membership; an empty non-nil slice
	// empties the group.
	ClientIDs []string

	IsActive *bool
}

// List fetches every client group in one request (GET /api/v1/crm/group/ —
// the deprecated singular route, which returns a bare JSON array rather
// than a paginated envelope).
//
// Deprecated: the singular list route is frozen for back-compat. Use
// ListPage, which targets the canonical cursor-paginated /api/v1/crm/groups/
// route and returns a *Page[ClientGroup] you can walk (or drain lazily with
// Page.All). This List remains source- and behavior-compatible and is not
// going away, but new code should prefer ListPage.
func (s *ClientGroupsService) List(ctx context.Context) ([]ClientGroup, error) {
	var out []ClientGroup
	if err := s.client.get(ctx, clientGroupsLegacyPath, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListPage returns one cursor-paginated page of client groups, newest
// first by id (GET /api/v1/crm/groups/ — the canonical plural route). Walk
// further pages with Page.NextPage, or drain them all lazily with Page.All.
func (s *ClientGroupsService) ListPage(ctx context.Context, params ClientGroupListParams) (*Page[ClientGroup], error) {
	return fetchPage[ClientGroup](ctx, s.client, clientGroupsPath, params.values())
}

// Create adds a client group (POST /api/v1/crm/groups/).
func (s *ClientGroupsService) Create(ctx context.Context, params ClientGroupCreateParams) (*ClientGroup, error) {
	body := map[string]any{"name": params.Name, "slug": params.Slug}
	if params.ClientIDs != nil {
		body["client_ids"] = params.ClientIDs
	}
	if params.IsActive != nil {
		body["is_active"] = *params.IsActive
	}
	var out ClientGroup
	if err := s.client.post(ctx, clientGroupsPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retrieve fetches one client group by id (GET /api/v1/crm/groups/{id}/).
func (s *ClientGroupsService) Retrieve(ctx context.Context, groupID int) (*ClientGroup, error) {
	var out ClientGroup
	if err := s.client.get(ctx, clientGroupsPath+strconv.Itoa(groupID)+"/", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update partially updates a client group
// (PATCH /api/v1/crm/groups/{id}/). A non-nil ClientIDs replaces the whole
// membership.
func (s *ClientGroupsService) Update(ctx context.Context, groupID int, params ClientGroupUpdateParams) (*ClientGroup, error) {
	body := map[string]any{}
	if params.Name != nil {
		body["name"] = *params.Name
	}
	if params.Slug != nil {
		body["slug"] = *params.Slug
	}
	if params.ClientIDs != nil {
		body["client_ids"] = params.ClientIDs
	}
	if params.IsActive != nil {
		body["is_active"] = *params.IsActive
	}
	var out ClientGroup
	if err := s.client.patch(ctx, clientGroupsPath+strconv.Itoa(groupID)+"/", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Replace fully replaces a client group (PUT /api/v1/crm/groups/{id}/).
func (s *ClientGroupsService) Replace(ctx context.Context, groupID int, params ClientGroupReplaceParams) (*ClientGroup, error) {
	body := map[string]any{
		"name":       params.Name,
		"slug":       params.Slug,
		"client_ids": params.ClientIDs,
	}
	if params.ClientIDs == nil {
		body["client_ids"] = []string{}
	}
	if params.IsActive != nil {
		body["is_active"] = *params.IsActive
	}
	var out ClientGroup
	if err := s.client.put(ctx, clientGroupsPath+strconv.Itoa(groupID)+"/", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a client group (DELETE /api/v1/crm/groups/{id}/, 204 on
// success). Member client profiles are not deleted.
func (s *ClientGroupsService) Delete(ctx context.Context, groupID int) error {
	return s.client.delete(ctx, clientGroupsPath+strconv.Itoa(groupID)+"/")
}
