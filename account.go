package silon

import (
	"context"
)

const (
	profilePath = "/api/v1/profile/"
	signupPath  = "/api/v1/signup/"
)

// UserProfile is the authenticated user's own profile — the body of
// GET /api/v1/profile/ and the created profile echoed by
// POST /api/v1/signup/.
type UserProfile struct {
	// Email is the user's email address. Also the login username.
	Email string `json:"email"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`

	// PhoneNumber is in E.164 format (e.g. "+96512345678").
	PhoneNumber string `json:"phone_number"`

	// CivilID is the Kuwait Civil ID; nullable.
	CivilID *string `json:"civil_id,omitempty"`

	// DefaultLanguage is the two-letter code (e.g. "en", "ar").
	DefaultLanguage string `json:"default_language,omitempty"`

	// ClientID is the linked contact profile's client_id (read-only).
	ClientID string `json:"client_id,omitempty"`
}

// ProfileService reads and updates the authenticated user's own profile
// (/api/v1/profile/). Access it via Client.Profile.
type ProfileService struct {
	client *Client
}

// ProfileUpdateParams are the parameters for ProfileService.Update
// (PATCH) — only non-nil fields are sent.
type ProfileUpdateParams struct {
	Email       *string
	FirstName   *string
	LastName    *string
	PhoneNumber *string

	// CivilID is the Kuwait Civil ID; validated and must be unique when
	// present.
	CivilID *string

	// DefaultLanguage is the two-letter code (e.g. "en", "ar").
	DefaultLanguage *string
}

func (p ProfileUpdateParams) body() map[string]any {
	body := map[string]any{}
	if p.Email != nil {
		body["email"] = *p.Email
	}
	if p.FirstName != nil {
		body["first_name"] = *p.FirstName
	}
	if p.LastName != nil {
		body["last_name"] = *p.LastName
	}
	if p.PhoneNumber != nil {
		body["phone_number"] = *p.PhoneNumber
	}
	if p.CivilID != nil {
		body["civil_id"] = *p.CivilID
	}
	if p.DefaultLanguage != nil {
		body["default_language"] = *p.DefaultLanguage
	}
	return body
}

// ProfileReplaceParams are the parameters for ProfileService.Replace
// (PUT) — the full new state of the profile. Email, FirstName,
// LastName and PhoneNumber are required; nil fields are omitted from
// the request JSON.
type ProfileReplaceParams struct {
	Email       string
	FirstName   string
	LastName    string
	PhoneNumber string

	// CivilID is the Kuwait Civil ID; validated and must be unique when
	// present.
	CivilID *string

	// DefaultLanguage is the two-letter code (e.g. "en", "ar").
	DefaultLanguage *string
}

func (p ProfileReplaceParams) body() map[string]any {
	body := map[string]any{
		"email":        p.Email,
		"first_name":   p.FirstName,
		"last_name":    p.LastName,
		"phone_number": p.PhoneNumber,
	}
	if p.CivilID != nil {
		body["civil_id"] = *p.CivilID
	}
	if p.DefaultLanguage != nil {
		body["default_language"] = *p.DefaultLanguage
	}
	return body
}

// Retrieve fetches the authenticated user's profile
// (GET /api/v1/profile/).
func (s *ProfileService) Retrieve(ctx context.Context) (*UserProfile, error) {
	var out UserProfile
	if err := s.client.get(ctx, profilePath, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update partially updates the authenticated user's profile
// (PATCH /api/v1/profile/) — only non-nil fields change.
func (s *ProfileService) Update(ctx context.Context, params ProfileUpdateParams) (*UserProfile, error) {
	var out UserProfile
	if err := s.client.patch(ctx, profilePath, params.body(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Replace fully replaces the authenticated user's profile
// (PUT /api/v1/profile/).
func (s *ProfileService) Replace(ctx context.Context, params ProfileReplaceParams) (*UserProfile, error) {
	var out UserProfile
	if err := s.client.put(ctx, profilePath, params.body(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AuthService signs up users. Access it via Client.Auth.
type AuthService struct {
	client *Client
}

// SignupParams are the parameters for AuthService.Signup. Email,
// FirstName, LastName, PhoneNumber and Password are required; nil
// fields are omitted from the request JSON.
type SignupParams struct {
	// Email is the new user's email address. Also the login username.
	Email string

	FirstName string
	LastName  string

	// PhoneNumber is in E.164 format (e.g. "+96512345678").
	PhoneNumber string

	// Password for the new account (write-only).
	Password string

	// CivilID is the Kuwait Civil ID; validated and must be unique when
	// present.
	CivilID *string

	// DefaultLanguage is the two-letter code (e.g. "en", "ar").
	DefaultLanguage *string

	// ClientID is an optional client id for the auto-created contact
	// profile; defaults to "KMS<id>" server-side when omitted.
	ClientID *string
}

func (p SignupParams) body() map[string]any {
	body := map[string]any{
		"email":        p.Email,
		"first_name":   p.FirstName,
		"last_name":    p.LastName,
		"phone_number": p.PhoneNumber,
		"password":     p.Password,
	}
	if p.CivilID != nil {
		body["civil_id"] = *p.CivilID
	}
	if p.DefaultLanguage != nil {
		body["default_language"] = *p.DefaultLanguage
	}
	if p.ClientID != nil {
		body["client_id"] = *p.ClientID
	}
	return body
}

// Signup creates a new user account (POST /api/v1/signup/, 201 — the
// body is the created profile; throttled server-side).
func (s *AuthService) Signup(ctx context.Context, params SignupParams) (*UserProfile, error) {
	var out UserProfile
	if err := s.client.post(ctx, signupPath, params.body(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
