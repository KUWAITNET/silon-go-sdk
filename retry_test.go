package silon

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

var acceptedMessage = map[string]any{
	"id": "m1", "object": "message", "channel": "sms", "status": "queued",
}

func TestGetRetriedOn500ThenSucceeds(t *testing.T) {
	m := newMockAPI(t, sequence(
		jsonStub(500, map[string]any{}),
		jsonStub(200, map[string]any{"event_id": "evt-1", "is_sent": true}),
	))
	c := newTestClient(t, m, WithMaxRetries(2))
	sleeps := captureSleeps(c)

	status, err := c.Messages.Retrieve(t.Context(), "evt-1")
	if err != nil {
		t.Fatal(err)
	}
	if !status.IsSent {
		t.Error("IsSent = false")
	}
	if m.callCount() != 2 {
		t.Errorf("calls = %d, want 2", m.callCount())
	}
	if len(*sleeps) != 1 {
		t.Errorf("sleeps = %v, want exactly one", *sleeps)
	}
}

func TestRetryHonoursRetryAfter(t *testing.T) {
	m := newMockAPI(t, sequence(
		jsonStubH(429, map[string]any{}, map[string]string{"Retry-After": "3"}),
		jsonStub(200, map[string]any{"event_id": "evt-1", "is_sent": true}),
	))
	c := newTestClient(t, m, WithMaxRetries(2))
	sleeps := captureSleeps(c)

	if _, err := c.Messages.Retrieve(t.Context(), "evt-1"); err != nil {
		t.Fatal(err)
	}
	if m.callCount() != 2 {
		t.Errorf("calls = %d, want 2", m.callCount())
	}
	if len(*sleeps) != 1 || (*sleeps)[0] < 3*time.Second {
		t.Errorf("sleeps = %v, want first >= 3s", *sleeps)
	}
}

func TestConnectionErrorRetried(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{"event_id": "evt-1", "is_sent": true})))
	transport := &flakyTransport{failures: 1, err: errors.New("boom")}
	c := newTestClient(t, m,
		WithMaxRetries(2),
		WithHTTPClient(&http.Client{Transport: transport}),
	)
	captureSleeps(c)

	if _, err := c.Messages.Retrieve(t.Context(), "evt-1"); err != nil {
		t.Fatal(err)
	}
	if transport.callCount() != 2 {
		t.Errorf("transport calls = %d, want 2", transport.callCount())
	}
}

func TestRetriesExhaustedReturnsLastError(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(500, map[string]any{})))
	c := newTestClient(t, m, WithMaxRetries(1))
	captureSleeps(c)

	_, err := c.Messages.Retrieve(t.Context(), "evt-1")
	if !IsInternalServer(err) {
		t.Fatalf("want 5xx APIError, got %v", err)
	}
	if m.callCount() != 2 { // initial attempt + 1 retry
		t.Errorf("calls = %d, want 2", m.callCount())
	}
}

func TestConnectionErrorsExhausted(t *testing.T) {
	cause := errors.New("down")
	transport := &flakyTransport{failures: 99, err: cause}
	m := newMockAPI(t, always(jsonStub(200, map[string]any{})))
	c := newTestClient(t, m,
		WithMaxRetries(1),
		WithHTTPClient(&http.Client{Transport: transport}),
	)
	captureSleeps(c)

	_, err := c.Messages.Retrieve(t.Context(), "evt-1")
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("want *ConnectionError, got %T (%v)", err, err)
	}
	if connErr.Timeout {
		t.Error("Timeout = true for a plain connection failure")
	}
	if !errors.Is(err, cause) {
		t.Error("Unwrap chain must reach the transport error")
	}
	if transport.callCount() != 2 {
		t.Errorf("transport calls = %d, want 2", transport.callCount())
	}
}

func TestTimeoutWithoutRetries(t *testing.T) {
	transport := &flakyTransport{failures: 99, err: timeoutError{}}
	m := newMockAPI(t, always(jsonStub(200, map[string]any{})))
	c := newTestClient(t, m, WithHTTPClient(&http.Client{Transport: transport})) // maxRetries 0

	_, err := c.Messages.Retrieve(t.Context(), "evt-1")
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("want *ConnectionError, got %T (%v)", err, err)
	}
	if !connErr.Timeout {
		t.Error("Timeout = false, want true")
	}
	if connErr.Error() != "Request timed out." {
		t.Errorf("Error() = %q", connErr.Error())
	}
}

func TestMaxRetriesZeroNeverRetries(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(500, map[string]any{})))
	c := newTestClient(t, m) // maxRetries 0
	sleeps := captureSleeps(c)

	_, err := c.Messages.Retrieve(t.Context(), "evt-1")
	if !IsInternalServer(err) {
		t.Fatalf("want 5xx APIError, got %v", err)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d, want 1", m.callCount())
	}
	if len(*sleeps) != 0 {
		t.Errorf("sleeps = %v, want none", *sleeps)
	}
}

func TestKeyedPostRetriedWithSameKey(t *testing.T) {
	m := newMockAPI(t, sequence(
		jsonStub(503, map[string]any{}),
		jsonStub(202, acceptedMessage),
	))
	c := newTestClient(t, m, WithMaxRetries(2))
	captureSleeps(c)

	sent := mustSend(t, c, MessageSendParams{
		Channel: "sms",
		To:      map[string]any{"phone_number": "+1"},
		Content: map[string]any{"body": "hi"},
	})
	if sent.ID != "m1" {
		t.Errorf("ID = %q", sent.ID)
	}
	if m.callCount() != 2 {
		t.Fatalf("calls = %d, want 2", m.callCount())
	}
	// The same Idempotency-Key must be replayed so the send cannot double-fire.
	first := m.call(0).header.Get("Idempotency-Key")
	second := m.call(1).header.Get("Idempotency-Key")
	if first == "" || first != second {
		t.Errorf("Idempotency-Key differs across attempts: %q vs %q", first, second)
	}
}

func TestPlainPostNeverRetried(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(500, map[string]any{})))
	c := newTestClient(t, m, WithMaxRetries(2))
	sleeps := captureSleeps(c)

	err := c.post(t.Context(), "/api/v1/crm/client/", map[string]any{"client_id": "c1"}, nil, nil)
	if !IsInternalServer(err) {
		t.Fatalf("want 5xx APIError, got %v", err)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d, want 1 (plain POST must not be retried)", m.callCount())
	}
	if len(*sleeps) != 0 {
		t.Errorf("sleeps = %v, want none", *sleeps)
	}
}

func TestNonRetryableStatusNotRetried(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(401, map[string]any{})))
	c := newTestClient(t, m, WithMaxRetries(2))
	captureSleeps(c)

	_, err := c.Messages.Retrieve(t.Context(), "evt-1")
	if !IsAuthentication(err) {
		t.Fatalf("want 401 APIError, got %v", err)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d, want 1", m.callCount())
	}
}

func TestBackoffGrowsWithAttempts(t *testing.T) {
	m := newMockAPI(t, sequence(
		jsonStub(500, map[string]any{}),
		jsonStub(500, map[string]any{}),
		jsonStub(200, map[string]any{"event_id": "evt-1", "is_sent": true}),
	))
	c := newTestClient(t, m, WithMaxRetries(2))
	sleeps := captureSleeps(c)

	if _, err := c.Messages.Retrieve(t.Context(), "evt-1"); err != nil {
		t.Fatal(err)
	}
	if len(*sleeps) != 2 {
		t.Fatalf("sleeps = %v, want 2", *sleeps)
	}
	// attempt 0: 0.5s..0.75s, attempt 1: 1.0s..1.25s — strictly increasing.
	if (*sleeps)[1] <= (*sleeps)[0] {
		t.Errorf("backoff did not grow: %v", *sleeps)
	}
	if (*sleeps)[0] < 500*time.Millisecond || (*sleeps)[0] > 750*time.Millisecond {
		t.Errorf("first delay %v outside [0.5s, 0.75s]", (*sleeps)[0])
	}
	if (*sleeps)[1] < time.Second || (*sleeps)[1] > 1250*time.Millisecond {
		t.Errorf("second delay %v outside [1s, 1.25s]", (*sleeps)[1])
	}
}

func TestSleepRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	start := time.Now()
	err := sleepContext(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if time.Since(start) > time.Second {
		t.Error("sleepContext did not return promptly on cancellation")
	}
}

func TestRetryLoopAbortsWhenContextCancelledDuringSleep(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(500, map[string]any{})))
	c := newTestClient(t, m, WithMaxRetries(3)) // default sleeper: real, ctx-aware

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := c.Messages.Retrieve(ctx, "evt-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("retry loop kept sleeping for %v after cancellation", elapsed)
	}
}
