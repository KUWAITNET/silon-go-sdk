package silon

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// B2 scheduling + cancellation: SendAt on Messages.Send / Broadcasts.Create /
// the file form of Messages.SendBatch, and the Cancel operations.

const scheduledEventID = "1767120000.1a2b3c-3f1c9e7a-2b4d-4c1e-9a3f-6d2b5c8e1f04"

var scheduledMessage = map[string]any{
	"id":       scheduledEventID,
	"object":   "message",
	"livemode": true,
	"channel":  "sms",
	"status":   "scheduled",
}

var canceledMessage = map[string]any{
	"id":       scheduledEventID,
	"object":   "message",
	"livemode": true,
	"channel":  "sms",
	"status":   "canceled",
}

var scheduledBroadcast = map[string]any{
	"id":            "br_01J9",
	"object":        "broadcast",
	"livemode":      true,
	"channel":       "email",
	"status":        "scheduled",
	"target_count":  nil, // audience resolves at dispatch time
	"skipped_count": nil,
}

var canceledBroadcast = map[string]any{
	"id":            "br_01J9",
	"object":        "broadcast",
	"livemode":      true,
	"channel":       "email",
	"status":        "canceled",
	"target_count":  nil,
	"skipped_count": nil,
}

func TestSendSendAtSerializesISO8601(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, scheduledMessage)))
	c := newTestClient(t, m)

	// A time.Time always carries an offset — serialized ISO-8601 with it.
	sendAt := time.Date(2026, 8, 15, 9, 30, 0, 0, time.FixedZone("AST", 3*60*60))
	sent := mustSend(t, c, MessageSendParams{
		Channel: "sms",
		To:      map[string]any{"phone_number": "+96550001234"},
		Content: map[string]any{"body": "Doors open at 10."},
		SendAt:  Time(sendAt),
	})
	if sent.Status != "scheduled" {
		t.Errorf("Status = %q, want scheduled", sent.Status)
	}
	if sent.ID != scheduledEventID {
		t.Errorf("ID = %q, want the stable scheduled id", sent.ID)
	}

	body := m.lastCall(t).jsonBody(t)
	if body["send_at"] != "2026-08-15T09:30:00+03:00" {
		t.Errorf("send_at = %v, want ISO-8601 with the value's own UTC offset", body["send_at"])
	}
	// Scheduled creates stay always-keyed, exactly like immediate ones.
	if m.lastCall(t).header.Get("Idempotency-Key") == "" {
		t.Error("Idempotency-Key missing: scheduled creates must stay keyed")
	}
}

func TestSendSendAtStringPassthrough(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, scheduledMessage)))
	c := newTestClient(t, m)

	// Pre-formatted ISO-8601 strings ride ExtraBody verbatim (merged last).
	mustSend(t, c, MessageSendParams{
		Channel:   "sms",
		To:        map[string]any{"phone_number": "+96550001234"},
		Content:   map[string]any{"body": "hi"},
		ExtraBody: map[string]any{"send_at": "2026-09-01T00:00:00Z"},
	})
	body := m.lastCall(t).jsonBody(t)
	if body["send_at"] != "2026-09-01T00:00:00Z" {
		t.Errorf("send_at = %v, want the string passed through verbatim", body["send_at"])
	}
}

func TestBroadcastCreateSendAtSerializesISO8601(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, scheduledBroadcast)))
	c := newTestClient(t, m)

	sendAt := time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC)
	created := mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:  "email",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
		Content:  map[string]any{"subject": "Launch", "body": "<h1>Soon</h1>"},
		SendAt:   Time(sendAt),
	})
	if created.Status != "scheduled" {
		t.Errorf("Status = %q, want scheduled", created.Status)
	}
	// target_count/skipped_count may be null until the audience resolves
	// at dispatch time — JSON null decodes as 0.
	if created.TargetCount != 0 || created.SkippedCount != 0 {
		t.Errorf("counts = %d/%d, want 0/0 for null", created.TargetCount, created.SkippedCount)
	}

	body := m.lastCall(t).jsonBody(t)
	if body["send_at"] != "2026-07-20T18:00:00Z" {
		t.Errorf("send_at = %v, want ISO-8601 UTC", body["send_at"])
	}
	if m.lastCall(t).header.Get("Idempotency-Key") == "" {
		t.Error("Idempotency-Key missing: scheduled creates must stay keyed")
	}
}

func TestSendBatchFileFormSendAt(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "1044", "object": "batch", "livemode": true, "status": "scheduled",
	})))
	c := newTestClient(t, m)

	sendAt := time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC)
	accepted := mustSendBatch(t, c, MessageBatchParams{
		File:    String("a1b2c3d4.csv"),
		Channel: String("sms"),
		Content: map[string]any{"body": "Hello {{name}}"},
		SendAt:  Time(sendAt),
	})
	if accepted.Status != "scheduled" {
		t.Errorf("Status = %q, want scheduled (expansion + send run at dispatch)", accepted.Status)
	}

	want := map[string]any{
		"file":    "a1b2c3d4.csv",
		"channel": "sms",
		"content": map[string]any{"body": "Hello {{name}}"},
		"send_at": "2026-08-01T06:00:00Z",
	}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestMessagesCancel(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, canceledMessage)))
	c := newTestClient(t, m)

	canceled, err := c.Messages.Cancel(t.Context(), scheduledEventID)
	if err != nil {
		t.Fatalf("Messages.Cancel: %v", err)
	}
	if canceled.ID != scheduledEventID || canceled.Object != "message" ||
		canceled.Channel != "sms" || canceled.Status != "canceled" {
		t.Errorf("canceled = %+v", canceled)
	}
	if !canceled.Livemode {
		t.Error("Livemode = false, want true on a live cancel")
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/messages/"+scheduledEventID+"/cancel/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if got := last.header.Get("Authorization"); got != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q", got)
	}
	// Cancel is idempotent by nature: no request body, no Idempotency-Key.
	if len(last.body) != 0 {
		t.Errorf("body = %q, want empty", last.body)
	}
	if key := last.header.Get("Idempotency-Key"); key != "" {
		t.Errorf("Idempotency-Key = %q, want none on cancel", key)
	}

	// Repeat cancels are safe: an already-canceled send answers 200 with
	// the canceled envelope again (NOT 409; no second event).
	again, err := c.Messages.Cancel(t.Context(), scheduledEventID)
	if err != nil {
		t.Fatalf("repeat Messages.Cancel: %v", err)
	}
	if again.Status != "canceled" || again.ID != canceled.ID {
		t.Errorf("repeat cancel = %+v, want the same canceled envelope", again)
	}
}

func TestMessagesCancelNotCancellable409(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(409, map[string]any{
		"type":   "https://acme.silon.tech/docs/errors/not-cancellable",
		"title":  "Not cancellable",
		"status": 409,
		"detail": "The message has already dispatched and can no longer be canceled.",
	})))
	c := newTestClient(t, m)

	_, err := c.Messages.Cancel(t.Context(), scheduledEventID)
	if !IsConflict(err) {
		t.Fatalf("want 409 APIError, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T", err)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0].Code != "not-cancellable" {
		t.Errorf("Errors = %+v, want code not-cancellable", apiErr.Errors)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d, want 1 (unkeyed POST must not be retried)", m.callCount())
	}
}

func TestBroadcastsCancel(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, canceledBroadcast)))
	c := newTestClient(t, m)

	canceled, err := c.Broadcasts.Cancel(t.Context(), "br_01J9")
	if err != nil {
		t.Fatalf("Broadcasts.Cancel: %v", err)
	}
	if canceled.ID != "br_01J9" || canceled.Object != "broadcast" || canceled.Status != "canceled" {
		t.Errorf("canceled = %+v", canceled)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/broadcasts/br_01J9/cancel/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if got := last.header.Get("Authorization"); got != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q", got)
	}
	if len(last.body) != 0 {
		t.Errorf("body = %q, want empty", last.body)
	}
	if key := last.header.Get("Idempotency-Key"); key != "" {
		t.Errorf("Idempotency-Key = %q, want none on cancel", key)
	}
}

func TestBroadcastsCancelNotCancellable409(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(409, map[string]any{
		"type":   "https://acme.silon.tech/docs/errors/not-cancellable",
		"title":  "Not cancellable",
		"status": 409,
		"detail": "The broadcast has already dispatched and can no longer be canceled.",
	})))
	c := newTestClient(t, m)

	_, err := c.Broadcasts.Cancel(t.Context(), "br_01J9")
	if !IsConflict(err) {
		t.Fatalf("want 409 APIError, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T", err)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0].Code != "not-cancellable" {
		t.Errorf("Errors = %+v, want code not-cancellable", apiErr.Errors)
	}
}

func TestRetrieveScheduledMessageStatus(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"event_id": scheduledEventID,
		"is_sent":  false,
		"livemode": true,
		"status":   "scheduled",
		"send_at":  "2026-08-15T06:30:00Z",
		"messages": []any{},
	})))
	c := newTestClient(t, m)

	// The scheduled id resolves on the status endpoint before dispatch.
	status, err := c.Messages.Retrieve(t.Context(), scheduledEventID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "scheduled" || status.IsSent {
		t.Errorf("status = %+v, want scheduled and not sent", status)
	}
	want := time.Date(2026, 8, 15, 6, 30, 0, 0, time.UTC)
	if status.SendAt == nil || !status.SendAt.Equal(want) {
		t.Errorf("SendAt = %v, want %v", status.SendAt, want)
	}
}

func TestBroadcastRetrieveScheduled(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"id":           "br_01J9",
		"channel":      "email",
		"livemode":     true,
		"target_count": nil,
		"queued":       0,
		"sent":         0,
		"failed":       0,
		"started_at":   nil,
		"completed_at": nil,
		"status":       "scheduled",
		"send_at":      "2026-07-20T18:00:00Z",
	})))
	c := newTestClient(t, m)

	broadcast, err := c.Broadcasts.Retrieve(t.Context(), "br_01J9")
	if err != nil {
		t.Fatal(err)
	}
	if broadcast.Status != "scheduled" {
		t.Errorf("Status = %q, want scheduled", broadcast.Status)
	}
	want := time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC)
	if broadcast.SendAt == nil || !broadcast.SendAt.Equal(want) {
		t.Errorf("SendAt = %v, want %v", broadcast.SendAt, want)
	}
	if broadcast.TargetCount != 0 {
		t.Errorf("TargetCount = %d, want 0 for null before dispatch", broadcast.TargetCount)
	}
}
