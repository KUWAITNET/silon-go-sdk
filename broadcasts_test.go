package silon

import (
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"
)

var broadcastJSON = map[string]any{
	"id":           "br_01J1",
	"channel":      "email",
	"livemode":     true,
	"target_count": 100,
	"queued":       0,
	"sent":         97,
	"failed":       3,
	"started_at":   "2026-07-01T10:00:00Z",
	"completed_at": "2026-07-01T10:05:00Z",
	"status":       "completed",
}

const deliveriesPath = "/api/v1/broadcasts/br_01J1/deliveries/"

var acceptedBroadcastCreate = map[string]any{
	"id":            "br_01J2",
	"object":        "broadcast",
	"livemode":      true,
	"channel":       "sms",
	"status":        "queued",
	"target_count":  2,
	"skipped_count": 1,
}

func mustCreateBroadcast(t *testing.T, c *Client, params BroadcastCreateParams) *BroadcastAccepted {
	t.Helper()
	created, err := c.Broadcasts.Create(t.Context(), params)
	if err != nil {
		t.Fatalf("Broadcasts.Create: %v", err)
	}
	return created
}

func deliveryJSON(n int, status string) map[string]any {
	return map[string]any{
		"id":        "d-" + string(rune('0'+n)),
		"client_id": "cust_00" + string(rune('0'+n)),
		"status":    status,
		"sent_at":   "2026-07-01T10:01:00Z",
		"error":     nil,
	}
}

// deliveriesResponder serves two pages of delivery rows; the advertised
// next URL points at a foreign host to prove it is never followed.
func deliveriesResponder(n int, c call) stub {
	switch c.query.Get("cursor") {
	case "":
		return jsonStub(200, map[string]any{
			"results":  []any{deliveryJSON(1, "sent"), deliveryJSON(2, "sent")},
			"next":     "https://internal-proxy.local" + deliveriesPath + "?cursor=pg2&limit=2",
			"previous": nil,
		})
	case "pg2":
		return jsonStub(200, map[string]any{
			"results": []any{map[string]any{
				"id":        "d-3",
				"client_id": "",
				"status":    "failed",
				"sent_at":   nil,
				"error":     "Mailbox unavailable",
			}},
			"next":     nil,
			"previous": "https://internal-proxy.local" + deliveriesPath + "?limit=2",
		})
	default:
		return jsonStub(404, map[string]any{})
	}
}

func TestBroadcastRetrieve(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, broadcastJSON)))
	c := newTestClient(t, m)

	broadcast, err := c.Broadcasts.Retrieve(t.Context(), "br_01J1")
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/broadcasts/br_01J1/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if broadcast.ID != "br_01J1" || broadcast.Channel != "email" || broadcast.Status != "completed" {
		t.Errorf("broadcast = %+v", broadcast)
	}
	if broadcast.TargetCount != 100 || broadcast.Sent != 97 || broadcast.Failed != 3 || broadcast.Queued != 0 {
		t.Errorf("counts = %+v", broadcast)
	}
	if broadcast.Livemode == nil || !*broadcast.Livemode {
		t.Errorf("Livemode = %v, want true on a live broadcast", broadcast.Livemode)
	}
	want := time.Date(2026, 7, 1, 10, 5, 0, 0, time.UTC)
	if broadcast.CompletedAt == nil || !broadcast.CompletedAt.Equal(want) {
		t.Errorf("CompletedAt = %v, want %v", broadcast.CompletedAt, want)
	}
}

func TestBroadcastInProgressHasNilCompletedAt(t *testing.T) {
	body := map[string]any{}
	for k, v := range broadcastJSON {
		body[k] = v
	}
	body["queued"] = 40
	body["completed_at"] = nil
	body["status"] = "in_progress"
	m := newMockAPI(t, always(jsonStub(200, body)))
	c := newTestClient(t, m)

	broadcast, err := c.Broadcasts.Retrieve(t.Context(), "br_01J1")
	if err != nil {
		t.Fatal(err)
	}
	if broadcast.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil", broadcast.CompletedAt)
	}
	if broadcast.Status != "in_progress" || broadcast.Queued != 40 {
		t.Errorf("broadcast = %+v", broadcast)
	}
}

func TestBroadcastDeliveriesParamsForwarded(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	_, err := c.Broadcasts.Deliveries(t.Context(), "br_01J1",
		BroadcastDeliveriesParams{Cursor: String("abc"), Limit: Int(50)})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.path != deliveriesPath {
		t.Errorf("path = %q, want %q", last.path, deliveriesPath)
	}
	if last.query.Get("cursor") != "abc" || last.query.Get("limit") != "50" {
		t.Errorf("query = %v", last.query)
	}
}

func TestBroadcastDeliveriesManualNextPageMergesParams(t *testing.T) {
	m := newMockAPI(t, deliveriesResponder)
	c := newTestClient(t, m)

	page, err := c.Broadcasts.Deliveries(t.Context(), "br_01J1",
		BroadcastDeliveriesParams{Limit: Int(2)})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 2 || !page.HasNextPage() {
		t.Fatalf("page = %+v", page)
	}
	if page.Results[0].ClientID != "cust_001" || page.Results[0].Status != "sent" {
		t.Errorf("Results[0] = %+v", page.Results[0])
	}

	page2, err := page.NextPage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Results) != 1 || page2.HasNextPage() {
		t.Fatalf("page2 = %+v", page2)
	}
	failed := page2.Results[0]
	if failed.Status != "failed" || failed.Error == nil || *failed.Error != "Mailbox unavailable" {
		t.Errorf("failed row = %+v", failed)
	}
	if failed.SentAt != nil {
		t.Errorf("SentAt = %v, want nil", failed.SentAt)
	}
	// The cursor from the opaque next URL merges over the original params,
	// and the request stays on the original path + configured base URL.
	second := m.lastCall(t)
	if second.path != deliveriesPath {
		t.Errorf("path = %q, want %q (foreign next host must not be followed)", second.path, deliveriesPath)
	}
	if second.query.Get("cursor") != "pg2" || second.query.Get("limit") != "2" {
		t.Errorf("second request query = %v", second.query)
	}
}

func TestBroadcastDeliveriesAll(t *testing.T) {
	m := newMockAPI(t, deliveriesResponder)
	c := newTestClient(t, m)

	page, err := c.Broadcasts.Deliveries(t.Context(), "br_01J1", BroadcastDeliveriesParams{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for delivery, err := range page.All(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, delivery.ID)
	}
	if len(ids) != 3 || ids[0] != "d-1" || ids[2] != "d-3" {
		t.Errorf("ids = %v", ids)
	}
	if m.callCount() != 2 {
		t.Errorf("calls = %d, want 2", m.callCount())
	}
}

func TestBroadcastCreateMinimal(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBroadcastCreate)))
	c := newTestClient(t, m)

	created := mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:  "sms",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
		Content:  map[string]any{"body": "Flash sale ends tonight"},
	})
	if created.ID != "br_01J2" || created.Object != "broadcast" ||
		created.Channel != "sms" || created.Status != "queued" {
		t.Errorf("created = %+v", created)
	}
	if created.TargetCount != 2 || created.SkippedCount != 1 {
		t.Errorf("counts = %+v", created)
	}
	if !created.Livemode {
		t.Error("Livemode = false, want true on a live broadcast create")
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/broadcasts/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if ct := last.header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	want := map[string]any{
		"channel":  "sms",
		"audience": map[string]any{"type": "client_group", "slug": "vip"},
		"content":  map[string]any{"body": "Flash sale ends tonight"},
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v (null fields must be omitted)", got, want)
	}
}

func TestBroadcastCreateDecodesTestModeLivemodeFalse(t *testing.T) {
	body := map[string]any{}
	for k, v := range acceptedBroadcastCreate {
		body[k] = v
	}
	body["livemode"] = false
	m := newMockAPI(t, always(jsonStub(202, body)))
	c := newTestClient(t, m)

	created := mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:  "sms",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
		Content:  map[string]any{"body": "test-mode broadcast"},
	})
	if created.Livemode {
		t.Error("Livemode = true, want false on a test-mode broadcast create")
	}
}

func TestBroadcastRetrieveDecodesTestModeLivemodeFalse(t *testing.T) {
	body := map[string]any{}
	for k, v := range broadcastJSON {
		body[k] = v
	}
	body["livemode"] = false
	m := newMockAPI(t, always(jsonStub(200, body)))
	c := newTestClient(t, m)

	broadcast, err := c.Broadcasts.Retrieve(t.Context(), "br_01J1")
	if err != nil {
		t.Fatal(err)
	}
	if broadcast.Livemode == nil || *broadcast.Livemode {
		t.Errorf("Livemode = %v, want false on a test-mode broadcast", broadcast.Livemode)
	}
}

func TestBroadcastCreateRecipientsAudienceVerbatim(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBroadcastCreate)))
	c := newTestClient(t, m)

	audience := map[string]any{
		"type": "recipients",
		"recipients": []any{
			map[string]any{"phone_number": "+96550001234"},
			map[string]any{"phone_number": "+96550001235"},
			map[string]any{"client_id": "cust_001"},
		},
	}
	mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:  "sms",
		Audience: audience,
		Content:  map[string]any{"body": "hi"},
	})

	body := m.lastCall(t).jsonBody(t)
	if !reflect.DeepEqual(body["audience"], audience) {
		t.Errorf("audience = %v, want it passed through verbatim", body["audience"])
	}
	if _, present := body["to"]; present {
		t.Error("'to' must be absent on a broadcast create")
	}
}

func TestBroadcastCreateAutoGeneratesIdempotencyKey(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBroadcastCreate)))
	c := newTestClient(t, m)
	mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:  "sms",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
		Content:  map[string]any{"body": "x"},
	})
	key := m.lastCall(t).header.Get("Idempotency-Key")
	uuidV4 := regexp.MustCompile(
		`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidV4.MatchString(key) {
		t.Errorf("Idempotency-Key = %q, want a v4 UUID", key)
	}
}

func TestBroadcastCreateExplicitIdempotencyKey(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBroadcastCreate)))
	c := newTestClient(t, m)
	mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:        "sms",
		Audience:       map[string]any{"type": "client_group", "slug": "vip"},
		Content:        map[string]any{"body": "x"},
		IdempotencyKey: "my-key-2",
	})
	if got := m.lastCall(t).header.Get("Idempotency-Key"); got != "my-key-2" {
		t.Errorf("Idempotency-Key = %q", got)
	}
}

func TestBroadcastCreateRetriedWithSameKey(t *testing.T) {
	m := newMockAPI(t, sequence(
		jsonStub(500, map[string]any{}),
		jsonStub(202, acceptedBroadcastCreate),
	))
	c := newTestClient(t, m, WithMaxRetries(2))
	captureSleeps(c)

	created := mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:  "sms",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
		Content:  map[string]any{"body": "hi"},
	})
	if created.ID != "br_01J2" {
		t.Errorf("ID = %q", created.ID)
	}
	if m.callCount() != 2 {
		t.Fatalf("calls = %d, want 2 (keyed POST must be retried)", m.callCount())
	}
	// The same Idempotency-Key must be replayed so the send cannot double-fire.
	first := m.call(0).header.Get("Idempotency-Key")
	second := m.call(1).header.Get("Idempotency-Key")
	if first == "" || first != second {
		t.Errorf("Idempotency-Key differs across attempts: %q vs %q", first, second)
	}
}

func TestBroadcastCreateChannelSpecificFields(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBroadcastCreate)))
	c := newTestClient(t, m)
	template := map[string]any{
		"name":      "order_confirmed",
		"language":  "en",
		"variables": map[string]any{"body_1": "Sara"},
	}
	mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:          "whatsapp",
		Audience:         map[string]any{"type": "client_ids", "client_ids": []any{"cust_001"}},
		WhatsAppTemplate: template,
		Provider:         String("meta_cloud"),
		Sender:           String("acme"),
		Application:      String("consumer-app"),
		Priority:         String("high"),
		TTL:              Int(3600),
	})
	body := m.lastCall(t).jsonBody(t)
	if body["provider"] != "meta_cloud" || body["sender"] != "acme" ||
		body["application"] != "consumer-app" || body["priority"] != "high" {
		t.Errorf("body = %v", body)
	}
	if body["ttl"] != float64(3600) {
		t.Errorf("ttl = %v (%T)", body["ttl"], body["ttl"])
	}
	if !reflect.DeepEqual(body["whatsapp_template"], template) {
		t.Errorf("whatsapp_template = %v", body["whatsapp_template"])
	}
	if _, present := body["content"]; present {
		t.Error("'content' must be absent when not provided")
	}
}

func TestBroadcastCreateExtraBodyPassthrough(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, acceptedBroadcastCreate)))
	c := newTestClient(t, m)
	mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:  "web_push",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
		Content:  map[string]any{"body": "hi"},
		Priority: String("low"),
		ExtraBody: map[string]any{
			"priority":     "high", // merged last: overrides the typed field
			"future_field": map[string]any{"nested": true},
		},
	})
	body := m.lastCall(t).jsonBody(t)
	if body["priority"] != "high" {
		t.Errorf("priority = %v, want ExtraBody to win on key collision", body["priority"])
	}
	if !reflect.DeepEqual(body["future_field"], map[string]any{"nested": true}) {
		t.Errorf("future_field = %v", body["future_field"])
	}
}

func TestBroadcastCreateUnknownResponseFieldsTolerated(t *testing.T) {
	body := map[string]any{"brand_new_field": "42"}
	for k, v := range acceptedBroadcastCreate {
		body[k] = v
	}
	m := newMockAPI(t, always(jsonStub(202, body)))
	c := newTestClient(t, m)

	created := mustCreateBroadcast(t, c, BroadcastCreateParams{
		Channel:  "sms",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
		Content:  map[string]any{"body": "x"},
	})
	if created.ID != "br_01J2" || created.TargetCount != 2 {
		t.Errorf("known fields must still deserialize: %+v", created)
	}
}

func TestBroadcastCreate422ProblemDecodes(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(422, map[string]any{
		"type":   "https://acme.silon.tech/docs/errors/audience-too-large",
		"title":  "Audience too large",
		"status": 422,
		"detail": "audience.recipients supports at most 1,000 rows.",
		"field":  "audience.recipients",
	})))
	c := newTestClient(t, m)

	_, err := c.Broadcasts.Create(t.Context(), BroadcastCreateParams{
		Channel:  "sms",
		Audience: map[string]any{"type": "recipients", "recipients": []any{}},
		Content:  map[string]any{"body": "x"},
	})
	if !IsUnprocessableEntity(err) {
		t.Fatalf("want 422 APIError, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T", err)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0].Code != "audience-too-large" {
		t.Errorf("Errors = %+v, want code audience-too-large", apiErr.Errors)
	}
	if apiErr.Errors[0].Attr == nil || *apiErr.Errors[0].Attr != "audience.recipients" {
		t.Errorf("Attr = %v, want audience.recipients", apiErr.Errors[0].Attr)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d, want 1 (422 must not be retried)", m.callCount())
	}
}
