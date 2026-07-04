package silon

import (
	"testing"
	"time"
)

var fullSuppressionJSON = map[string]any{
	"id":       "sup_1a2b3c4d",
	"object":   "suppression",
	"address":  "+96550001234",
	"channel":  "sms",
	"reason":   "stop",
	"livemode": true,
	"created":  "2026-07-01T10:00:00Z",
}

func suppressionJSON(n int) map[string]any {
	return map[string]any{
		"id":       "sup_" + string(rune('0'+n)),
		"object":   "suppression",
		"address":  "user" + string(rune('0'+n)) + "@example.com",
		"channel":  nil,
		"reason":   "unsubscribe",
		"livemode": true,
		"created":  "2026-07-01T10:00:00Z",
	}
}

// suppressionsTwoPageResponder serves page 1 (no cursor) then page 2
// (cursor=abc), with the next URL on a foreign host to prove pagination
// stays on the configured base URL.
func suppressionsTwoPageResponder(n int, c call) stub {
	switch c.query.Get("cursor") {
	case "":
		return jsonStub(200, map[string]any{
			"results":  []any{suppressionJSON(1), suppressionJSON(2)},
			"next":     "https://internal-proxy.local" + suppressionsPath + "?cursor=abc&limit=2",
			"previous": nil,
		})
	case "abc":
		return jsonStub(200, map[string]any{
			"results":  []any{suppressionJSON(3)},
			"next":     nil,
			"previous": "https://internal-proxy.local" + suppressionsPath + "?limit=2",
		})
	default:
		return jsonStub(404, map[string]any{})
	}
}

func TestSuppressionsListTrailingSlashAndFilters(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{fullSuppressionJSON}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	page, err := c.Suppressions.List(t.Context(), SuppressionListParams{
		Address: String("+96550001234"),
		Channel: String("sms"),
		Reason:  String(SuppressionReasonStop),
		Limit:   Int(10),
	})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/suppressions/" {
		t.Errorf("%s %s (path must have a trailing slash)", last.method, last.path)
	}
	if last.query.Get("address") != "+96550001234" || last.query.Get("channel") != "sms" ||
		last.query.Get("reason") != "stop" || last.query.Get("limit") != "10" {
		t.Errorf("query = %v", last.query)
	}
	if len(page.Results) != 1 {
		t.Fatalf("len(Results) = %d", len(page.Results))
	}
	got := page.Results[0]
	if got.ID != "sup_1a2b3c4d" || got.Object != "suppression" ||
		got.Address != "+96550001234" || got.Reason != "stop" || !got.Livemode {
		t.Errorf("suppression = %+v", got)
	}
	if got.Channel == nil || *got.Channel != "sms" {
		t.Errorf("Channel = %v", got.Channel)
	}
	wantCreated := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	if got.Created == nil || !got.Created.Equal(wantCreated) {
		t.Errorf("Created = %v", got.Created)
	}
}

func TestSuppressionsListOmitsNilFilters(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"results": []any{}, "next": nil, "previous": nil,
	})))
	c := newTestClient(t, m)

	if _, err := c.Suppressions.List(t.Context(), SuppressionListParams{}); err != nil {
		t.Fatal(err)
	}
	if q := m.lastCall(t).query; len(q) != 0 {
		t.Errorf("query = %v, want empty", q)
	}
}

func TestSuppressionsListManualNextPage(t *testing.T) {
	m := newMockAPI(t, suppressionsTwoPageResponder)
	c := newTestClient(t, m)

	page, err := c.Suppressions.List(t.Context(), SuppressionListParams{Limit: Int(2)})
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasNextPage() {
		t.Fatal("HasNextPage() = false")
	}
	page2, err := page.NextPage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Results) != 1 || page2.Results[0].ID != "sup_3" {
		t.Errorf("page2.Results = %+v", page2.Results)
	}
	if page2.HasNextPage() {
		t.Error("HasNextPage() = true on the last page")
	}
	// The cursor from the opaque next URL merges over the original params
	// and the request stays on the configured base URL and path (the
	// foreign host in the next URL is never followed).
	second := m.lastCall(t)
	if second.path != suppressionsPath {
		t.Errorf("path = %q, want %q", second.path, suppressionsPath)
	}
	if second.query.Get("cursor") != "abc" || second.query.Get("limit") != "2" {
		t.Errorf("second request query = %v", second.query)
	}
}

func TestSuppressionsListAutoPaging(t *testing.T) {
	m := newMockAPI(t, suppressionsTwoPageResponder)
	c := newTestClient(t, m)

	page, err := c.Suppressions.List(t.Context(), SuppressionListParams{Limit: Int(2)})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for row, err := range page.All(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, row.ID)
	}
	if len(ids) != 3 || ids[0] != "sup_1" || ids[2] != "sup_3" {
		t.Errorf("ids = %v", ids)
	}
	if m.callCount() != 2 {
		t.Errorf("calls = %d, want 2", m.callCount())
	}
}

func TestSuppressionsCreate(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, fullSuppressionJSON)))
	c := newTestClient(t, m)

	created, err := c.Suppressions.Create(t.Context(), SuppressionCreateParams{
		Address: "+965 5000 1234",
		Channel: String("sms"),
		Reason:  String(SuppressionReasonStop),
	})
	if err != nil {
		t.Fatal(err)
	}
	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/suppressions/" {
		t.Errorf("%s %s (path must have a trailing slash)", last.method, last.path)
	}
	body := last.jsonBody(t)
	if body["address"] != "+965 5000 1234" || body["channel"] != "sms" || body["reason"] != "stop" {
		t.Errorf("body = %v", body)
	}
	// Create is idempotent by nature (duplicate -> 200 with the existing
	// row), so it must never send an Idempotency-Key.
	if got := last.header.Get("Idempotency-Key"); got != "" {
		t.Errorf("Idempotency-Key = %q, want none on suppression create", got)
	}
	if created.ID != "sup_1a2b3c4d" || created.Address != "+96550001234" {
		t.Errorf("created = %+v", created)
	}
}

func TestSuppressionsCreateMinimalOmitsOptionals(t *testing.T) {
	body := map[string]any{}
	for k, v := range fullSuppressionJSON {
		body[k] = v
	}
	body["channel"] = nil
	body["reason"] = "manual"
	m := newMockAPI(t, always(jsonStub(201, body)))
	c := newTestClient(t, m)

	created, err := c.Suppressions.Create(t.Context(), SuppressionCreateParams{
		Address: "sara@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	sent := m.lastCall(t).jsonBody(t)
	if len(sent) != 1 || sent["address"] != "sara@example.com" {
		t.Errorf("body = %v, want only address", sent)
	}
	if created.Channel != nil {
		t.Errorf("Channel = %v, want nil (all channels)", created.Channel)
	}
	if created.Reason != "manual" {
		t.Errorf("Reason = %q", created.Reason)
	}
}

func TestSuppressionsCreateDuplicateAnswers200WithExisting(t *testing.T) {
	// A duplicate (address, channel) in the same mode is idempotent by
	// nature: 200 with the EXISTING object, never an error.
	m := newMockAPI(t, always(jsonStub(200, fullSuppressionJSON)))
	c := newTestClient(t, m)

	existing, err := c.Suppressions.Create(t.Context(), SuppressionCreateParams{
		Address: "+96550001234",
		Channel: String("sms"),
	})
	if err != nil {
		t.Fatalf("duplicate create must not error: %v", err)
	}
	if existing.ID != "sup_1a2b3c4d" || existing.Reason != "stop" {
		t.Errorf("existing = %+v", existing)
	}
}

func TestSuppressionsDelete(t *testing.T) {
	m := newMockAPI(t, always(stub{status: 204}))
	c := newTestClient(t, m)

	if err := c.Suppressions.Delete(t.Context(), "sup_1a2b3c4d"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	last := m.lastCall(t)
	if last.method != "DELETE" || last.path != "/api/v1/suppressions/sup_1a2b3c4d/" {
		t.Errorf("%s %s (path must have a trailing slash)", last.method, last.path)
	}
}

func TestSuppressionUnknownFieldTolerance(t *testing.T) {
	body := map[string]any{}
	for k, v := range fullSuppressionJSON {
		body[k] = v
	}
	body["livemode"] = false
	body["future_field"] = map[string]any{"nested": true}
	body["another"] = []any{1, 2, 3}
	m := newMockAPI(t, always(jsonStub(201, body)))
	c := newTestClient(t, m)

	created, err := c.Suppressions.Create(t.Context(), SuppressionCreateParams{
		Address: "+96550001234",
	})
	if err != nil {
		t.Fatalf("unknown response fields must be ignored: %v", err)
	}
	if created.ID != "sup_1a2b3c4d" {
		t.Errorf("ID = %q", created.ID)
	}
	if created.Livemode {
		t.Error("Livemode = true, want false on a test-mode row")
	}
}

func TestBroadcastCreateDecodesSkippedBreakdown(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "br_01J1", "object": "broadcast", "channel": "sms",
		"status": "queued", "livemode": true,
		"target_count": 7, "skipped_count": 6,
		"skipped": map[string]any{"suppressed": 3, "wrong_channel": 2, "duplicate": 1},
	})))
	c := newTestClient(t, m)

	created, err := c.Broadcasts.Create(t.Context(), BroadcastCreateParams{
		Channel:  "sms",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.SkippedCount != 6 {
		t.Errorf("SkippedCount = %d", created.SkippedCount)
	}
	if created.Skipped == nil {
		t.Fatal("Skipped = nil, want the breakdown")
	}
	if created.Skipped.Suppressed != 3 || created.Skipped.WrongChannel != 2 || created.Skipped.Duplicate != 1 {
		t.Errorf("Skipped = %+v", created.Skipped)
	}
}

func TestBroadcastCreateToleratesAbsentSkippedBreakdown(t *testing.T) {
	// Servers predating the breakdown send skipped_count only.
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "br_01J1", "object": "broadcast", "channel": "sms",
		"status": "queued", "livemode": true,
		"target_count": 2, "skipped_count": 1,
	})))
	c := newTestClient(t, m)

	created, err := c.Broadcasts.Create(t.Context(), BroadcastCreateParams{
		Channel:  "sms",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Skipped != nil {
		t.Errorf("Skipped = %+v, want nil when the server omits it", created.Skipped)
	}
	if created.SkippedCount != 1 {
		t.Errorf("SkippedCount = %d", created.SkippedCount)
	}
}

func TestMessageSendDecodesSkippedBreakdown(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "br_01J1", "object": "broadcast", "channel": "sms",
		"status": "queued", "livemode": true,
		"target_count": 9, "skipped_count": 2,
		"skipped": map[string]any{"suppressed": 2, "wrong_channel": 0, "duplicate": 0},
	})))
	c := newTestClient(t, m)

	sent := mustSend(t, c, MessageSendParams{
		Channel:  "sms",
		Audience: map[string]any{"type": "client_group", "slug": "vip"},
	})
	if sent.Skipped == nil {
		t.Fatal("Skipped = nil, want the breakdown")
	}
	if sent.Skipped.Suppressed != 2 || sent.Skipped.WrongChannel != 0 || sent.Skipped.Duplicate != 0 {
		t.Errorf("Skipped = %+v", sent.Skipped)
	}
}

func TestMessageSendToleratesAbsentSkippedBreakdown(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "msg_1", "object": "message", "channel": "sms",
		"status": "queued", "livemode": true,
	})))
	c := newTestClient(t, m)

	sent := mustSend(t, c, MessageSendParams{
		Channel: "sms",
		To:      map[string]any{"phone_number": "+96550001234"},
	})
	if sent.Skipped != nil {
		t.Errorf("Skipped = %+v, want nil on a single-recipient envelope", sent.Skipped)
	}
}

func TestSendBatchInlineDecodesSkippedBreakdown(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "batch_1", "object": "batch", "livemode": true,
		"messages": []any{
			map[string]any{"id": "msg_1", "object": "message", "channel": "sms", "status": "queued"},
		},
		"skipped_count": 1,
		"skipped":       map[string]any{"suppressed": 1, "wrong_channel": 0, "duplicate": 0},
	})))
	c := newTestClient(t, m)

	batch, err := c.Messages.SendBatch(t.Context(), MessageBatchParams{
		Channel: String("sms"),
		Messages: []map[string]any{
			{"to": map[string]any{"phone_number": "+96550001234"}},
			{"to": map[string]any{"phone_number": "+15005550009"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The suppressed row is omitted from the per-row envelopes and counted
	// in the breakdown instead.
	if len(batch.Messages) != 1 {
		t.Errorf("len(Messages) = %d", len(batch.Messages))
	}
	if batch.SkippedCount == nil || *batch.SkippedCount != 1 {
		t.Errorf("SkippedCount = %v", batch.SkippedCount)
	}
	if batch.Skipped == nil || batch.Skipped.Suppressed != 1 {
		t.Errorf("Skipped = %+v", batch.Skipped)
	}
}

func TestSendBatchToleratesAbsentSkippedBreakdown(t *testing.T) {
	// The file form (and servers predating the field) omit both.
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "bulk_42", "object": "batch", "livemode": true, "status": "queued",
	})))
	c := newTestClient(t, m)

	batch, err := c.Messages.SendBatch(t.Context(), MessageBatchParams{
		File:    String("recipients.csv"),
		Channel: String("sms"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.SkippedCount != nil || batch.Skipped != nil {
		t.Errorf("SkippedCount = %v, Skipped = %+v, want both nil", batch.SkippedCount, batch.Skipped)
	}
}

func TestOverrideSuppressionSerializes(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "msg_1", "object": "message", "channel": "email",
		"status": "queued", "livemode": true,
	})))
	c := newTestClient(t, m)

	mustSend(t, c, MessageSendParams{
		Channel:             "email",
		To:                  map[string]any{"email": "sara@example.com"},
		Content:             map[string]any{"subject": "Receipt", "body": "..."},
		OverrideSuppression: Bool(true),
	})
	body := m.lastCall(t).jsonBody(t)
	if body["override_suppression"] != true {
		t.Errorf("override_suppression = %v, want true", body["override_suppression"])
	}
}

func TestOverrideSuppressionOmittedWhenNil(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, map[string]any{
		"id": "msg_1", "object": "message", "channel": "sms",
		"status": "queued", "livemode": true,
	})))
	c := newTestClient(t, m)

	mustSend(t, c, MessageSendParams{
		Channel: "sms",
		To:      map[string]any{"phone_number": "+96550001234"},
	})
	body := m.lastCall(t).jsonBody(t)
	if _, present := body["override_suppression"]; present {
		t.Errorf("override_suppression must be omitted when unset: %v", body)
	}
}
