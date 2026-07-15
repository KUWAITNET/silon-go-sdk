package silon

import (
	"reflect"
	"testing"
)

var profileResponse = map[string]any{
	"email":            "sara@example.com",
	"first_name":       "Sara",
	"last_name":        "Ahmad",
	"phone_number":     "+96512345678",
	"civil_id":         "",
	"default_language": "en",
	"client_id":        "KMS42",
}

func TestProfileRetrieve(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, profileResponse)))
	c := newTestClient(t, m)

	profile, err := c.Profile.Retrieve(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if profile.Email != "sara@example.com" || profile.ClientID != "KMS42" {
		t.Errorf("profile = %+v", profile)
	}
	if profile.CivilID == nil || *profile.CivilID != "" {
		t.Errorf("CivilID = %v, want pointer to empty string", profile.CivilID)
	}

	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/profile/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}

func TestProfileUpdatePatch(t *testing.T) {
	updated := map[string]any{}
	for k, v := range profileResponse {
		updated[k] = v
	}
	updated["first_name"] = "Noor"

	m := newMockAPI(t, always(jsonStub(200, updated)))
	c := newTestClient(t, m)

	profile, err := c.Profile.Update(t.Context(), ProfileUpdateParams{
		FirstName: String("Noor"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.FirstName != "Noor" {
		t.Errorf("FirstName = %q", profile.FirstName)
	}

	last := m.lastCall(t)
	if last.method != "PATCH" || last.path != "/api/v1/profile/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"first_name": "Noor"}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestProfileReplacePut(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, profileResponse)))
	c := newTestClient(t, m)

	_, err := c.Profile.Replace(t.Context(), ProfileReplaceParams{
		Email:       "sara@example.com",
		FirstName:   "Sara",
		LastName:    "Ahmad",
		PhoneNumber: "+96512345678",
	})
	if err != nil {
		t.Fatal(err)
	}

	last := m.lastCall(t)
	if last.method != "PUT" || last.path != "/api/v1/profile/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"email":        "sara@example.com",
		"first_name":   "Sara",
		"last_name":    "Ahmad",
		"phone_number": "+96512345678",
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestAuthSignup(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, profileResponse)))
	c := newTestClient(t, m)

	created, err := c.Auth.Signup(t.Context(), SignupParams{
		Email:       "sara@example.com",
		FirstName:   "Sara",
		LastName:    "Ahmad",
		PhoneNumber: "+96512345678",
		Password:    "s3cret!pass",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Email != "sara@example.com" || created.ClientID != "KMS42" {
		t.Errorf("created = %+v", created)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/signup/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	got := last.jsonBody(t)
	if got["password"] != "s3cret!pass" {
		t.Errorf("password = %v", got["password"])
	}
	if _, present := got["civil_id"]; present {
		t.Errorf("civil_id must be omitted when nil, body = %v", got)
	}
	want := map[string]any{
		"email":        "sara@example.com",
		"first_name":   "Sara",
		"last_name":    "Ahmad",
		"phone_number": "+96512345678",
		"password":     "s3cret!pass",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	if key := last.header.Get("Idempotency-Key"); key != "" {
		t.Errorf("signup must not send an Idempotency-Key, got %q", key)
	}
}
