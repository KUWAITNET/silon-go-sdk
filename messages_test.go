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
