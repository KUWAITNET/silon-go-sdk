package silon

import (
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var acceptedMessageWA = map[string]any{
	"id":      "9f3e8a82-1c5a-4b1f-9d4c-7b5d2c8f3e9a",
	"object":  "message",
	"channel": "whatsapp",
	"status":  "queued",
}

var acceptedBroadcast = map[string]any{
	"id":            "br_01J1",
	"object":        "broadcast",
	"channel":       "email",
	"status":        "queued",
	"target_count":  240,
	"skipped_count": 3,
}

func TestSendMinimal(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedMessageWA)))
	c := newTestClient(t, m)

	sent := mustSend(t, c, MessageSendParams{
		Channel: "whatsapp",
		To:      map[string]any{"client_id": "cust_001"},
		Content: map[string]any{"body": "Your order has shipped"},
	})
	if sent.Object != "message" || sent.Status != "queued" {
		t.Errorf("sent = %+v", sent)
	}
	if sent.TargetCount != nil || sent.SkippedCount != nil {
		t.Errorf("broadcast-only counts must be nil on a message: %+v", sent)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/messages/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if ct := last.header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	want := map[string]any{
		"channel": "whatsapp",
		"to":      map[string]any{"client_id": "cust_001"},
		"content": map[string]any{"body": "Your order has shipped"},
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v (null fields must be omitted)", got, want)
	}
}

func TestSendAutoGeneratesIdempotencyKey(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedMessageWA)))
	c := newTestClient(t, m)
	mustSend(t, c, MessageSendParams{
		Channel: "sms",
		To:      map[string]any{"phone_number": "+1"},
		Content: map[string]any{"body": "x"},
	})
	key := m.lastCall(t).header.Get("Idempotency-Key")
	uuidV4 := regexp.MustCompile(
		`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidV4.MatchString(key) {
		t.Errorf("Idempotency-Key = %q, want a v4 UUID", key)
	}
}

func TestSendExplicitIdempotencyKey(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedMessageWA)))
	c := newTestClient(t, m)
	mustSend(t, c, MessageSendParams{
		Channel:        "sms",
		To:             map[string]any{"phone_number": "+1"},
		Content:        map[string]any{"body": "x"},
		IdempotencyKey: "my-key-1",
	})
	if got := m.lastCall(t).header.Get("Idempotency-Key"); got != "my-key-1" {
		t.Errorf("Idempotency-Key = %q", got)
	}
}

func TestSendRequiresExactlyOneTarget(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedMessageWA)))
	c := newTestClient(t, m)

	_, err := c.Messages.Send(t.Context(), MessageSendParams{
		Channel: "sms",
		Content: map[string]any{"body": "x"},
	})
	var baseErr *Error
	if !errors.As(err, &baseErr) || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("neither target: err = %v, want *Error mentioning 'exactly one'", err)
	}

	_, err = c.Messages.Send(t.Context(), MessageSendParams{
		Channel:  "sms",
		To:       map[string]any{"client_id": "a"},
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
		Content:  map[string]any{"body": "x"},
	})
	if !errors.As(err, &baseErr) || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("both targets: err = %v, want *Error mentioning 'exactly one'", err)
	}
	if m.callCount() != 0 {
		t.Errorf("client-side validation must not hit the network (calls = %d)", m.callCount())
	}
}

func TestSendBroadcast(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBroadcast)))
	c := newTestClient(t, m)

	sent := mustSend(t, c, MessageSendParams{
		Channel:  "email",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
		Content:  map[string]any{"subject": "Hi", "body": "<h1>Hello</h1>"},
	})
	if sent.Object != "broadcast" {
		t.Errorf("Object = %q", sent.Object)
	}
	if sent.TargetCount == nil || *sent.TargetCount != 240 {
		t.Errorf("TargetCount = %v", sent.TargetCount)
	}
	if sent.SkippedCount == nil || *sent.SkippedCount != 3 {
		t.Errorf("SkippedCount = %v", sent.SkippedCount)
	}

	body := m.lastCall(t).jsonBody(t)
	wantAudience := map[string]any{"type": "client_group", "slug": "vip"}
	if !reflect.DeepEqual(body["audience"], wantAudience) {
		t.Errorf("audience = %v", body["audience"])
	}
	if _, present := body["to"]; present {
		t.Error("'to' must be absent on a broadcast")
	}
}

func TestSendChannelSpecificFields(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedMessageWA)))
	c := newTestClient(t, m)
	mustSend(t, c, MessageSendParams{
		Channel:     "push",
		To:          map[string]any{"client_id": "cust_001"},
		Content:     map[string]any{"body": "New episode"},
		Application: String("consumer-app"),
		Priority:    String("high"),
		TTL:         Int(3600),
		Provider:    String("fcm"),
		Sender:      String("acme"),
	})
	body := m.lastCall(t).jsonBody(t)
	if body["application"] != "consumer-app" || body["priority"] != "high" ||
		body["provider"] != "fcm" || body["sender"] != "acme" {
		t.Errorf("body = %v", body)
	}
	if body["ttl"] != float64(3600) {
		t.Errorf("ttl = %v (%T)", body["ttl"], body["ttl"])
	}
}

func TestSendWhatsAppTemplateBlock(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedMessageWA)))
	c := newTestClient(t, m)
	template := map[string]any{
		"name":      "order_confirmed",
		"language":  "en",
		"variables": map[string]any{"body_1": "Sara"},
	}
	mustSend(t, c, MessageSendParams{
		Channel:          "whatsapp",
		To:               map[string]any{"phone_number": "+12025550123"},
		WhatsAppTemplate: template,
		Provider:         String("meta_cloud"),
	})
	body := m.lastCall(t).jsonBody(t)
	if !reflect.DeepEqual(body["whatsapp_template"], template) {
		t.Errorf("whatsapp_template = %v", body["whatsapp_template"])
	}
	if _, present := body["content"]; present {
		t.Error("'content' must be absent when not provided")
	}
}

func TestSendExtraBodyPassthrough(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedMessageWA)))
	c := newTestClient(t, m)
	mustSend(t, c, MessageSendParams{
		Channel: "web_push",
		To:      map[string]any{"client_id": "cust_001"},
		Content: map[string]any{"body": "hi"},
		ExtraBody: map[string]any{
			"widget_key":   "wk_1",
			"future_field": map[string]any{"nested": true},
		},
	})
	body := m.lastCall(t).jsonBody(t)
	if body["widget_key"] != "wk_1" {
		t.Errorf("widget_key = %v", body["widget_key"])
	}
	if !reflect.DeepEqual(body["future_field"], map[string]any{"nested": true}) {
		t.Errorf("future_field = %v", body["future_field"])
	}
}

func TestRetrieveStatus(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"event_id": "evt-1",
		"is_sent":  true,
		"messages": []any{map[string]any{
			"client_id":    "cust_001",
			"phone_number": "+1",
			"email":        "",
			"is_read":      true,
			"read_count":   2,
		}},
	})))
	c := newTestClient(t, m)

	status, err := c.Messages.Retrieve(t.Context(), "evt-1")
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/messages/evt-1/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if !status.IsSent || status.EventID != "evt-1" {
		t.Errorf("status = %+v", status)
	}
	if len(status.Messages) != 1 || status.Messages[0].ReadCount != 2 || !status.Messages[0].IsRead {
		t.Errorf("Messages = %+v", status.Messages)
	}
}

var acceptedBatch = map[string]any{
	"id":     "batch_01J4",
	"object": "batch",
	"messages": []any{
		map[string]any{"id": "m-1", "object": "message", "channel": "sms", "status": "queued"},
		map[string]any{"id": "m-2", "object": "message", "channel": "email", "status": "queued"},
	},
}

func mustSendBatch(t *testing.T, c *Client, params MessageBatchParams) *BatchAccepted {
	t.Helper()
	accepted, err := c.Messages.SendBatch(t.Context(), params)
	if err != nil {
		t.Fatalf("Messages.SendBatch: %v", err)
	}
	return accepted
}

func TestSendBatchMinimal(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBatch)))
	c := newTestClient(t, m)

	rows := []map[string]any{
		{"channel": "sms", "to": map[string]any{"phone_number": "+96550001234"},
			"content": map[string]any{"body": "Sara, your table for 2 is confirmed."}},
		{"channel": "email", "to": map[string]any{"email": "omar@example.com"},
			"content": map[string]any{"subject": "Confirmed", "body": "Omar, see you at 9pm."}},
	}
	accepted := mustSendBatch(t, c, MessageBatchParams{Messages: rows})
	if accepted.ID != "batch_01J4" || accepted.Object != "batch" {
		t.Errorf("accepted = %+v", accepted)
	}
	// Per-row envelopes must come back in request order.
	if len(accepted.Messages) != 2 ||
		accepted.Messages[0].ID != "m-1" || accepted.Messages[1].ID != "m-2" {
		t.Fatalf("Messages = %+v", accepted.Messages)
	}
	first := accepted.Messages[0]
	if first.Object != "message" || first.Channel != "sms" || first.Status != "queued" {
		t.Errorf("Messages[0] = %+v", first)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/messages/batch/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if ct := last.header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	want := map[string]any{
		"messages": []any{
			map[string]any{"channel": "sms", "to": map[string]any{"phone_number": "+96550001234"},
				"content": map[string]any{"body": "Sara, your table for 2 is confirmed."}},
			map[string]any{"channel": "email", "to": map[string]any{"email": "omar@example.com"},
				"content": map[string]any{"subject": "Confirmed", "body": "Omar, see you at 9pm."}},
		},
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v (null fields must be omitted, rows verbatim)", got, want)
	}
}

func TestSendBatchTopLevelDefaultChannel(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBatch)))
	c := newTestClient(t, m)

	mustSendBatch(t, c, MessageBatchParams{
		Channel: String("sms"),
		Messages: []map[string]any{
			{"to": map[string]any{"phone_number": "+96550001234"},
				"content": map[string]any{"body": "hi"}},
		},
	})
	body := m.lastCall(t).jsonBody(t)
	if body["channel"] != "sms" {
		t.Errorf("channel = %v, want top-level default forwarded", body["channel"])
	}
}

func TestSendBatchAutoGeneratesIdempotencyKey(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBatch)))
	c := newTestClient(t, m)
	mustSendBatch(t, c, MessageBatchParams{
		Channel:  String("sms"),
		Messages: []map[string]any{{"to": map[string]any{"phone_number": "+1"}}},
	})
	key := m.lastCall(t).header.Get("Idempotency-Key")
	uuidV4 := regexp.MustCompile(
		`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidV4.MatchString(key) {
		t.Errorf("Idempotency-Key = %q, want a v4 UUID", key)
	}
}

func TestSendBatchExplicitIdempotencyKey(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBatch)))
	c := newTestClient(t, m)
	mustSendBatch(t, c, MessageBatchParams{
		Channel:        String("sms"),
		Messages:       []map[string]any{{"to": map[string]any{"phone_number": "+1"}}},
		IdempotencyKey: "my-key-3",
	})
	if got := m.lastCall(t).header.Get("Idempotency-Key"); got != "my-key-3" {
		t.Errorf("Idempotency-Key = %q", got)
	}
}

func TestSendBatchRetriedWithSameKey(t *testing.T) {
	m := newMockAPI(t, sequence(
		jsonStub(500, map[string]any{}),
		jsonStub(202, acceptedBatch),
	))
	c := newTestClient(t, m, WithMaxRetries(2))
	captureSleeps(c)

	accepted := mustSendBatch(t, c, MessageBatchParams{
		Channel:  String("sms"),
		Messages: []map[string]any{{"to": map[string]any{"phone_number": "+1"}}},
	})
	if accepted.ID != "batch_01J4" {
		t.Errorf("ID = %q", accepted.ID)
	}
	if m.callCount() != 2 {
		t.Fatalf("calls = %d, want 2 (keyed POST must be retried)", m.callCount())
	}
	// The same Idempotency-Key must be replayed so the batch cannot double-fire.
	first := m.call(0).header.Get("Idempotency-Key")
	second := m.call(1).header.Get("Idempotency-Key")
	if first == "" || first != second {
		t.Errorf("Idempotency-Key differs across attempts: %q vs %q", first, second)
	}
}

func TestSendBatchExtraBodyPassthrough(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBatch)))
	c := newTestClient(t, m)
	mustSendBatch(t, c, MessageBatchParams{
		Channel:  String("sms"),
		Messages: []map[string]any{{"to": map[string]any{"phone_number": "+1"}}},
		ExtraBody: map[string]any{
			"channel":      "email", // merged last: overrides the typed field
			"future_field": map[string]any{"nested": true},
		},
	})
	body := m.lastCall(t).jsonBody(t)
	if body["channel"] != "email" {
		t.Errorf("channel = %v, want ExtraBody to win on key collision", body["channel"])
	}
	if !reflect.DeepEqual(body["future_field"], map[string]any{"nested": true}) {
		t.Errorf("future_field = %v", body["future_field"])
	}
}

func TestSendBatchUnknownResponseFieldsTolerated(t *testing.T) {
	body := map[string]any{
		"brand_new_field": "42",
		"messages": []any{map[string]any{
			"id": "m-1", "object": "message", "channel": "sms",
			"status": "queued", "row_new_field": true,
		}},
	}
	for k, v := range acceptedBatch {
		if k != "messages" {
			body[k] = v
		}
	}
	m := newMockAPI(t, always(jsonStub(202, body)))
	c := newTestClient(t, m)

	accepted := mustSendBatch(t, c, MessageBatchParams{
		Channel:  String("sms"),
		Messages: []map[string]any{{"to": map[string]any{"phone_number": "+1"}}},
	})
	if accepted.ID != "batch_01J4" || len(accepted.Messages) != 1 ||
		accepted.Messages[0].ID != "m-1" {
		t.Errorf("known fields must still deserialize: %+v", accepted)
	}
}

func TestSendBatch422ProblemDecodes(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(422, map[string]any{
		"type":   "https://acme.silon.tech/docs/errors/validation-failed",
		"title":  "Validation failed",
		"status": 422,
		"detail": "Row 3 is invalid; nothing was queued.",
		"field":  "messages[3].to.phone_number",
	})))
	c := newTestClient(t, m)

	_, err := c.Messages.SendBatch(t.Context(), MessageBatchParams{
		Channel:  String("sms"),
		Messages: []map[string]any{{"to": map[string]any{"phone_number": "not-a-number"}}},
	})
	if !IsUnprocessableEntity(err) {
		t.Fatalf("want 422 APIError, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T", err)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0].Code != "validation-failed" {
		t.Errorf("Errors = %+v, want code validation-failed", apiErr.Errors)
	}
	if apiErr.Errors[0].Attr == nil || *apiErr.Errors[0].Attr != "messages[3].to.phone_number" {
		t.Errorf("Attr = %v, want the per-index path messages[3].to.phone_number", apiErr.Errors[0].Attr)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d, want 1 (422 must not be retried)", m.callCount())
	}
}

func TestSendBatchRequiresExactlyOneSource(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBatch)))
	c := newTestClient(t, m)

	_, err := c.Messages.SendBatch(t.Context(), MessageBatchParams{
		Channel: String("sms"),
	})
	var baseErr *Error
	if !errors.As(err, &baseErr) || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("neither source: err = %v, want *Error mentioning 'exactly one'", err)
	}

	_, err = c.Messages.SendBatch(t.Context(), MessageBatchParams{
		Messages: []map[string]any{{"to": map[string]any{"phone_number": "+1"}}},
		File:     String("a1b2c3.csv"),
	})
	if !errors.As(err, &baseErr) || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("both sources: err = %v, want *Error mentioning 'exactly one'", err)
	}
	if m.callCount() != 0 {
		t.Errorf("client-side validation must not hit the network (calls = %d)", m.callCount())
	}
}

func TestSendBatchFileForm(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id":        "1042",
		"object":    "batch",
		"status":    "queued",
		"row_count": 1200,
	})))
	c := newTestClient(t, m)

	accepted := mustSendBatch(t, c, MessageBatchParams{
		File:    String("a1b2c3d4.csv"),
		Channel: String("sms"),
		Content: map[string]any{"body": "Hello {{name}}"},
	})
	if accepted.ID != "1042" || accepted.Object != "batch" || accepted.Status != "queued" {
		t.Errorf("accepted = %+v", accepted)
	}
	if accepted.RowCount == nil || *accepted.RowCount != 1200 {
		t.Errorf("RowCount = %v, want 1200", accepted.RowCount)
	}
	// The file form has no per-row envelopes — Messages must be nil.
	if accepted.Messages != nil {
		t.Errorf("Messages = %+v, want nil on the file form", accepted.Messages)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/messages/batch/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"file":    "a1b2c3d4.csv",
		"channel": "sms",
		"content": map[string]any{"body": "Hello {{name}}"},
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want exactly %v (no 'messages' key, nil defaults omitted)", got, want)
	}
}

func TestSendBatchFileFormRowCountOmitted(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "1043", "object": "batch", "status": "queued",
	})))
	c := newTestClient(t, m)

	accepted := mustSendBatch(t, c, MessageBatchParams{File: String("a1b2c3d4.csv")})
	if accepted.RowCount != nil {
		t.Errorf("RowCount = %v, want nil when the server omits row_count", accepted.RowCount)
	}
	if accepted.Messages != nil {
		t.Errorf("Messages = %+v, want nil", accepted.Messages)
	}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, map[string]any{"file": "a1b2c3d4.csv"}) {
		t.Errorf("body = %v, want exactly {file}", got)
	}
}

func TestSendBatchInlineRowDefaults(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBatch)))
	c := newTestClient(t, m)

	rows := []map[string]any{{"to": map[string]any{"phone_number": "+96550001234"}}}
	mustSendBatch(t, c, MessageBatchParams{
		Messages:         rows,
		Channel:          String("whatsapp"),
		Content:          map[string]any{"body": "fallback"},
		Template:         map[string]any{"slug": "welcome"},
		Provider:         String("meta_cloud"),
		Sender:           String("ACME"),
		Application:      String("acme-app"),
		WidgetKey:        String("wk_1"),
		Priority:         String("high"),
		TTL:              Int(3600),
		WhatsApp:         map[string]any{"preview_url": true},
		WhatsAppTemplate: map[string]any{"name": "order_confirmed", "language": "en"},
	})
	want := map[string]any{
		"messages":          []any{map[string]any{"to": map[string]any{"phone_number": "+96550001234"}}},
		"channel":           "whatsapp",
		"content":           map[string]any{"body": "fallback"},
		"template":          map[string]any{"slug": "welcome"},
		"provider":          "meta_cloud",
		"sender":            "ACME",
		"application":       "acme-app",
		"widget_key":        "wk_1",
		"priority":          "high",
		"ttl":               float64(3600),
		"whatsapp":          map[string]any{"preview_url": true},
		"whatsapp_template": map[string]any{"name": "order_confirmed", "language": "en"},
	}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want all request-level row defaults serialized: %v", got, want)
	}
}

func TestUnknownResponseFieldsTolerated(t *testing.T) {
	body := map[string]any{"brand_new_field": "42"}
	for k, v := range acceptedMessageWA {
		body[k] = v
	}
	m := newMockAPI(t, always(jsonStub(202, body)))
	c := newTestClient(t, m)

	sent := mustSend(t, c, MessageSendParams{
		Channel: "sms",
		To:      map[string]any{"phone_number": "+1"},
		Content: map[string]any{"body": "x"},
	})
	if sent.ID != acceptedMessageWA["id"] || sent.Channel != "whatsapp" {
		t.Errorf("known fields must still deserialize: %+v", sent)
	}
}
