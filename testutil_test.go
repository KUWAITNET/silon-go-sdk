package silon

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

const testAPIKey = "sk_live_test"

// call is one request recorded by the mock API server.
type call struct {
	method string
	path   string
	query  url.Values
	header http.Header
	body   []byte
}

func (c call) jsonBody(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(c.body, &m); err != nil {
		t.Fatalf("request body is not a JSON object: %v\nbody: %s", err, c.body)
	}
	return m
}

// stub is one canned HTTP response.
type stub struct {
	status int
	body   []byte
	header map[string]string
}

func jsonStub(status int, v any) stub {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return stub{status: status, body: b, header: map[string]string{"Content-Type": "application/json"}}
}

func jsonStubH(status int, v any, header map[string]string) stub {
	s := jsonStub(status, v)
	for k, val := range header {
		s.header[k] = val
	}
	return s
}

func rawStub(status int, body string, header map[string]string) stub {
	return stub{status: status, body: []byte(body), header: header}
}

// mockAPI is an httptest.Server that records every request and answers
// from a respond function.
type mockAPI struct {
	server *httptest.Server

	mu    sync.Mutex
	calls []call
}

// newMockAPI starts a mock API server. respond receives the 1-based call
// number and the recorded call.
func newMockAPI(t *testing.T, respond func(n int, c call) stub) *mockAPI {
	t.Helper()
	m := &mockAPI{}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec := call{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			header: r.Header.Clone(),
			body:   body,
		}
		m.mu.Lock()
		m.calls = append(m.calls, rec)
		n := len(m.calls)
		m.mu.Unlock()

		st := respond(n, rec)
		for k, v := range st.header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(st.status)
		if len(st.body) > 0 {
			w.Write(st.body)
		}
	}))
	t.Cleanup(m.server.Close)
	return m
}

func (m *mockAPI) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockAPI) call(i int) call {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[i]
}

func (m *mockAPI) lastCall(t *testing.T) call {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		t.Fatal("mock API received no calls")
	}
	return m.calls[len(m.calls)-1]
}

// always answers every request with the same stub.
func always(st stub) func(int, call) stub {
	return func(int, call) stub { return st }
}

// sequence answers call n with stubs[n-1], repeating the final stub.
func sequence(stubs ...stub) func(int, call) stub {
	return func(n int, _ call) stub {
		if n > len(stubs) {
			n = len(stubs)
		}
		return stubs[n-1]
	}
}

// newTestClient builds a client against the mock server with retries
// disabled; trailing opts override the defaults.
func newTestClient(t *testing.T, m *mockAPI, opts ...Option) *Client {
	t.Helper()
	base := []Option{
		WithAPIKey(testAPIKey),
		WithBaseURL(m.server.URL),
		WithMaxRetries(0),
	}
	c, err := NewClient(append(base, opts...)...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// captureSleeps replaces the client's retry sleeper with a recorder.
func captureSleeps(c *Client) *[]time.Duration {
	recorded := &[]time.Duration{}
	c.sleep = func(_ context.Context, d time.Duration) error {
		*recorded = append(*recorded, d)
		return nil
	}
	return recorded
}

// flakyTransport fails the first `failures` round trips with err, then
// delegates to base.
type flakyTransport struct {
	failures int
	err      error
	base     http.RoundTripper

	mu    sync.Mutex
	calls int
}

func (f *flakyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.calls++
	n := f.calls
	f.mu.Unlock()
	if n <= f.failures {
		return nil, f.err
	}
	base := f.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func (f *flakyTransport) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// timeoutError satisfies net.Error's Timeout() so *url.Error classifies
// the failure as a timeout.
type timeoutError struct{}

func (timeoutError) Error() string   { return "deadline exceeded (mock)" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// clearSilonEnv blanks every SILON_* variable the client reads, so config
// tests are isolated from the developer's shell.
func clearSilonEnv(t *testing.T) {
	t.Helper()
	t.Setenv(envAPIKey, "")
	t.Setenv(envBaseURL, "")
	t.Setenv(envWorkspace, "")
}

func mustSend(t *testing.T, c *Client, params MessageSendParams) *MessageAccepted {
	t.Helper()
	sent, err := c.Messages.Send(t.Context(), params)
	if err != nil {
		t.Fatalf("Messages.Send: %v", err)
	}
	return sent
}
