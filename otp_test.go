package silon

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

var otpSendResponse = map[string]any{
	"otp_id":     "018f7c2e-0000-7000-8000-000000000001",
	"expires_at": "2026-07-02T12:05:00Z",
	"channel":    "sms",
	"livemode":   true,
	"task_ids":   []any{"t1"},
}

func TestOTPSend(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, otpSendResponse)))
	c := newTestClient(t, m)

	result, err := c.OTP.Send(t.Context(), OTPSendParams{
		Purpose: "login",
		To:      map[string]any{"phone_number": "+96512345678"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OTPID != "018f7c2e-0000-7000-8000-000000000001" || result.Channel != "sms" {
		t.Errorf("result = %+v", result)
	}
	wantExpiry := time.Date(2026, 7, 2, 12, 5, 0, 0, time.UTC)
	if !result.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", result.ExpiresAt, wantExpiry)
	}
	if len(result.TaskIDs) != 1 || result.TaskIDs[0] != "t1" {
		t.Errorf("TaskIDs = %v", result.TaskIDs)
	}
	if !result.Livemode {
		t.Error("Livemode = false, want true on a live OTP send")
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/otp/send/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"purpose": "login",
		"to":      map[string]any{"phone_number": "+96512345678"},
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	key := last.header.Get("Idempotency-Key")
	uuidV4 := regexp.MustCompile(
		`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidV4.MatchString(key) {
		t.Errorf("Idempotency-Key = %q, want a v4 UUID", key)
	}
}

func TestOTPSendExplicitIdempotencyKey(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(202, otpSendResponse)))
	c := newTestClient(t, m)

	_, err := c.OTP.Send(t.Context(), OTPSendParams{
		Purpose:        "login",
		To:             map[string]any{"email": "a@b.c"},
		IdempotencyKey: "otp-key-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.lastCall(t).header.Get("Idempotency-Key"); got != "otp-key-1" {
		t.Errorf("Idempotency-Key = %q", got)
	}
}

func TestOTPSendRetriedWithSameKey(t *testing.T) {
	m := newMockAPI(t, sequence(
		jsonStub(503, map[string]any{}),
		jsonStub(202, otpSendResponse),
	))
	c := newTestClient(t, m, WithMaxRetries(2))
	captureSleeps(c)

	result, err := c.OTP.Send(t.Context(), OTPSendParams{
		Purpose: "login",
		To:      map[string]any{"phone_number": "+1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Channel != "sms" {
		t.Errorf("Channel = %q", result.Channel)
	}
	if m.callCount() != 2 {
		t.Fatalf("calls = %d, want 2", m.callCount())
	}
	first := m.call(0).header.Get("Idempotency-Key")
	second := m.call(1).header.Get("Idempotency-Key")
	if first == "" || first != second {
		t.Errorf("Idempotency-Key differs across attempts: %q vs %q", first, second)
	}
}

func TestOTPVerifySuccess(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"verified":    true,
		"purpose":     "login",
		"livemode":    true,
		"verified_at": "2026-07-02T12:01:00Z",
	})))
	c := newTestClient(t, m)

	result, err := c.OTP.Verify(t.Context(), OTPVerifyParams{OTPID: "otp-1", Code: " 424242 "})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Verified || result.Purpose != "login" {
		t.Errorf("result = %+v", result)
	}
	if !result.Livemode {
		t.Error("Livemode = false, want true on a live OTP verify")
	}
	wantAt := time.Date(2026, 7, 2, 12, 1, 0, 0, time.UTC)
	if !result.VerifiedAt.Equal(wantAt) {
		t.Errorf("VerifiedAt = %v", result.VerifiedAt)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/otp/verify/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	// The code is passed through verbatim (no trimming client-side).
	want := map[string]any{"otp_id": "otp-1", "code": " 424242 "}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	if got := last.header.Get("Idempotency-Key"); got != "" {
		t.Errorf("verify must not send an Idempotency-Key, got %q", got)
	}
}

func TestOTPSendDecodesTestModeLivemodeFalse(t *testing.T) {
	body := map[string]any{}
	for k, v := range otpSendResponse {
		body[k] = v
	}
	body["livemode"] = false
	m := newMockAPI(t, always(jsonStub(202, body)))
	c := newTestClient(t, m)

	result, err := c.OTP.Send(t.Context(), OTPSendParams{
		Purpose: "login",
		To:      map[string]any{"phone_number": "+96512345678"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Livemode {
		t.Error("Livemode = true, want false on a test-mode OTP send")
	}
}

func TestOTPVerifyDecodesTestModeLivemodeFalse(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"verified":    true,
		"purpose":     "login",
		"livemode":    false,
		"verified_at": "2026-07-02T12:01:00Z",
	})))
	c := newTestClient(t, m)

	result, err := c.OTP.Verify(t.Context(), OTPVerifyParams{OTPID: "otp-1", Code: "000000"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Verified {
		t.Errorf("result = %+v", result)
	}
	if result.Livemode {
		t.Error("Livemode = true, want false on a test-mode OTP verify")
	}
}

func TestOTPVerifyWrongCodeExposesRemainingAttempts(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(400, map[string]any{
		"verified":           false,
		"remaining_attempts": 2,
	})))
	c := newTestClient(t, m)

	_, err := c.OTP.Verify(t.Context(), OTPVerifyParams{OTPID: "otp-1", Code: "000000"})
	if !IsBadRequest(err) {
		t.Fatalf("want 400 APIError, got %v", err)
	}
	apiErr := err.(*APIError)
	var failure struct {
		Verified          bool `json:"verified"`
		RemainingAttempts int  `json:"remaining_attempts"`
	}
	if unmarshalErr := json.Unmarshal(apiErr.Body, &failure); unmarshalErr != nil {
		t.Fatalf("Body is not the failure JSON: %v (body: %s)", unmarshalErr, apiErr.Body)
	}
	if failure.Verified || failure.RemainingAttempts != 2 {
		t.Errorf("failure body = %+v", failure)
	}
}

func TestOTPVerifyExpiredIsGone(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(410, map[string]any{
		"type":   "https://acme.silon.tech/docs/errors/otp-expired",
		"title":  "Gone",
		"status": 410,
		"detail": "This code has expired.",
		"field":  nil,
	})))
	c := newTestClient(t, m)

	_, err := c.OTP.Verify(t.Context(), OTPVerifyParams{OTPID: "otp-1", Code: "424242"})
	if !IsGone(err) {
		t.Fatalf("want 410 APIError, got %v", err)
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("message = %q, want it to mention 'expired'", err.Error())
	}
	apiErr := err.(*APIError)
	if len(apiErr.Errors) != 1 || apiErr.Errors[0].Code != "otp-expired" {
		t.Errorf("Errors = %+v", apiErr.Errors)
	}
}
