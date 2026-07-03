package silon

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// -- configuration ---------------------------------------------------------

func TestMissingAPIKeyReturnsError(t *testing.T) {
	clearSilonEnv(t)
	_, err := NewClient(WithBaseURL("https://acme.silon.tech"))
	var baseErr *Error
	if !errors.As(err, &baseErr) {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if !strings.Contains(baseErr.Message, "No API key") {
		t.Errorf("message = %q, want mention of missing API key", baseErr.Message)
	}
}

func TestMissingBaseURLReturnsError(t *testing.T) {
	clearSilonEnv(t)
	_, err := NewClient(WithAPIKey(testAPIKey))
	var baseErr *Error
	if !errors.As(err, &baseErr) {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if !strings.Contains(baseErr.Message, "No base URL") {
		t.Errorf("message = %q, want mention of missing base URL", baseErr.Message)
	}
}

func TestWorkspaceBuildsBaseURL(t *testing.T) {
	clearSilonEnv(t)
	c, err := NewClient(WithAPIKey(testAPIKey), WithWorkspace("acme"))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.BaseURL(); got != "https://acme.silon.tech" {
		t.Errorf("BaseURL() = %q", got)
	}
}

func TestExplicitBaseURLWinsOverWorkspace(t *testing.T) {
	clearSilonEnv(t)
	c, err := NewClient(
		WithAPIKey(testAPIKey),
		WithWorkspace("acme"),
		WithBaseURL("https://other.example"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.BaseURL(); got != "https://other.example" {
		t.Errorf("BaseURL() = %q", got)
	}
}

func TestEnvBaseURLBeatsWorkspaceArgument(t *testing.T) {
	clearSilonEnv(t)
	t.Setenv(envBaseURL, "https://env-url.example")
	c, err := NewClient(WithAPIKey(testAPIKey), WithWorkspace("acme"))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.BaseURL(); got != "https://env-url.example" {
		t.Errorf("BaseURL() = %q, want the SILON_BASE_URL value", got)
	}
}

func TestTrailingSlashStripped(t *testing.T) {
	clearSilonEnv(t)
	c, err := NewClient(WithAPIKey(testAPIKey), WithBaseURL("https://acme.silon.tech/"))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.BaseURL(); got != "https://acme.silon.tech" {
		t.Errorf("BaseURL() = %q", got)
	}
}

func TestEnvFallbacks(t *testing.T) {
	clearSilonEnv(t)
	t.Setenv(envAPIKey, "sk_live_env")
	t.Setenv(envWorkspace, "envspace")
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if c.apiKey != "sk_live_env" {
		t.Errorf("apiKey = %q", c.apiKey)
	}
	if got := c.BaseURL(); got != "https://envspace.silon.tech" {
		t.Errorf("BaseURL() = %q", got)
	}
}

func TestEnvBaseURLStripped(t *testing.T) {
	clearSilonEnv(t)
	t.Setenv(envAPIKey, "sk_live_env")
	t.Setenv(envBaseURL, "https://on-prem.example/")
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.BaseURL(); got != "https://on-prem.example" {
		t.Errorf("BaseURL() = %q", got)
	}
}

// -- transport -------------------------------------------------------------

func TestAuthAndUserAgentHeaders(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{"email": "a@b.c"})))
	c := newTestClient(t, m)
	if _, err := c.Messages.Retrieve(t.Context(), "evt-1"); err != nil {
		t.Fatal(err)
	}
	h := m.lastCall(t).header
	if got := h.Get("Authorization"); got != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q", got)
	}
	if got := h.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
	uaPattern := regexp.MustCompile(`^silon-go/` + regexp.QuoteMeta(Version) + ` go/\d`)
	if got := h.Get("User-Agent"); !uaPattern.MatchString(got) {
		t.Errorf("User-Agent = %q, want match for %s", got, uaPattern)
	}
}

func TestDefaultHeadersSent(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{})))
	c := newTestClient(t, m, WithDefaultHeader("X-Custom", "yes"))
	if _, err := c.Messages.Retrieve(t.Context(), "evt-1"); err != nil {
		t.Fatal(err)
	}
	if got := m.lastCall(t).header.Get("X-Custom"); got != "yes" {
		t.Errorf("X-Custom = %q", got)
	}
}

func Test204ReturnsVoid(t *testing.T) {
	m := newMockAPI(t, always(stub{status: 204}))
	c := newTestClient(t, m)
	out := map[string]any{"sentinel": true}
	err := c.do(t.Context(), requestSpec{method: http.MethodDelete, path: "/api/v1/crm/client/c1/"}, &out)
	if err != nil {
		t.Fatalf("DELETE 204: %v", err)
	}
	if _, ok := out["sentinel"]; !ok {
		t.Error("204 must leave the output untouched")
	}
}

func TestEmptyBodyReturnsVoid(t *testing.T) {
	m := newMockAPI(t, always(stub{status: 200}))
	c := newTestClient(t, m)
	var out map[string]any
	if err := c.do(t.Context(), requestSpec{method: http.MethodGet, path: "/api/v1/x/"}, &out); err != nil {
		t.Fatalf("empty 200: %v", err)
	}
	if out != nil {
		t.Errorf("out = %v, want untouched nil", out)
	}
}

func TestInvalidJSONSuccessBodyIsBaseError(t *testing.T) {
	m := newMockAPI(t, always(rawStub(200, "<html>not json</html>", map[string]string{"Content-Type": "text/html"})))
	c := newTestClient(t, m)
	_, err := c.Messages.Retrieve(t.Context(), "evt-1")
	var baseErr *Error
	if !errors.As(err, &baseErr) {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if !strings.Contains(baseErr.Message, "Could not parse response body") {
		t.Errorf("message = %q", baseErr.Message)
	}
}

func TestCustomHTTPClientUsed(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{})))
	custom := &http.Client{Timeout: 5 * time.Second}
	c := newTestClient(t, m, WithHTTPClient(custom))
	if c.httpClient != custom {
		t.Fatal("WithHTTPClient must be used as-is")
	}
	if _, err := c.Messages.Retrieve(t.Context(), "evt-1"); err != nil {
		t.Fatal(err)
	}
}

func TestWithTimeoutSetsInternalClientTimeout(t *testing.T) {
	clearSilonEnv(t)
	c, err := NewClient(
		WithAPIKey(testAPIKey),
		WithBaseURL("https://acme.silon.tech"),
		WithTimeout(7*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	if c.httpClient.Timeout != 7*time.Second {
		t.Errorf("Timeout = %v", c.httpClient.Timeout)
	}
}

func TestDefaultTimeoutIs30s(t *testing.T) {
	clearSilonEnv(t)
	c, err := NewClient(WithAPIKey(testAPIKey), WithBaseURL("https://acme.silon.tech"))
	if err != nil {
		t.Fatal(err)
	}
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", c.httpClient.Timeout)
	}
}

func TestNewUUIDIsV4(t *testing.T) {
	pattern := regexp.MustCompile(
		`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for range 32 {
		id := newUUID()
		if !pattern.MatchString(id) {
			t.Fatalf("newUUID() = %q, not a v4 UUID", id)
		}
		if seen[id] {
			t.Fatalf("newUUID() repeated %q", id)
		}
		seen[id] = true
	}
}
