package silon

import (
	"context"
	"time"
)

const (
	otpSendPath   = "/api/v1/otp/send/"
	otpVerifyPath = "/api/v1/otp/verify/"
)

// OTPService sends and verifies one-time passwords. Access it via
// Client.OTP.
type OTPService struct {
	client *Client
}

// OTPSendParams are the parameters for OTPService.Send.
type OTPSendParams struct {
	// Purpose names a configured OTP purpose on the tenant (e.g.
	// "login"); it decides the delivery channel, code shape and expiry.
	Purpose string

	// To targets the recipient: exactly one of "client_id" /
	// "phone_number" / "email".
	To map[string]any

	// IdempotencyKey is sent as the Idempotency-Key header. When empty, a
	// UUIDv4 is generated — the header is ALWAYS sent, and the same value
	// is replayed on every retry attempt, so a retry can never double-send.
	IdempotencyKey string
}

// OTPSendResult is the 202 body of POST /api/v1/otp/send/.
type OTPSendResult struct {
	// OTPID is the opaque id for this OTP; pass it back to
	// OTPService.Verify.
	OTPID string `json:"otp_id"`

	// ExpiresAt is when the code expires. Verifying after this returns a
	// 410 *APIError (IsGone).
	ExpiresAt time.Time `json:"expires_at"`

	// Channel is the channel the code was dispatched over (decided by the
	// purpose), e.g. "sms".
	Channel string `json:"channel"`

	// TaskIDs are tracking ids for the dispatched send(s); usually one.
	TaskIDs []string `json:"task_ids,omitempty"`
}

// OTPVerifyParams are the parameters for OTPService.Verify.
type OTPVerifyParams struct {
	// OTPID is the id returned by OTPService.Send.
	OTPID string

	// Code is the code the user entered.
	Code string
}

// OTPVerifyResult is the 200 body of POST /api/v1/otp/verify/.
type OTPVerifyResult struct {
	// Verified is true when the code matched and the OTP is now consumed.
	Verified bool `json:"verified"`

	// Purpose is the purpose the OTP was issued for.
	Purpose string `json:"purpose"`

	// VerifiedAt is when verification succeeded.
	VerifiedAt time.Time `json:"verified_at"`
}

// Send dispatches a one-time password (POST /api/v1/otp/send/, 202). The
// recipient in params.To must contain exactly one of "client_id" /
// "phone_number" / "email"; the delivery channel is decided by the
// configured purpose. An Idempotency-Key header is always sent
// (auto-generated UUIDv4 when params.IdempotencyKey is empty), which makes
// automatic retries safe.
func (s *OTPService) Send(ctx context.Context, params OTPSendParams) (*OTPSendResult, error) {
	key := params.IdempotencyKey
	if key == "" {
		key = newUUID()
	}
	body := map[string]any{"purpose": params.Purpose, "to": params.To}
	var out OTPSendResult
	if err := s.client.post(ctx, otpSendPath, body,
		map[string]string{"Idempotency-Key": key}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Verify checks a code (POST /api/v1/otp/verify/). A wrong code returns a
// 400 *APIError (IsBadRequest) whose Body carries {"verified": false,
// "remaining_attempts": N}; an expired or locked OTP returns a 410
// *APIError (IsGone).
func (s *OTPService) Verify(ctx context.Context, params OTPVerifyParams) (*OTPVerifyResult, error) {
	body := map[string]any{"otp_id": params.OTPID, "code": params.Code}
	var out OTPVerifyResult
	if err := s.client.post(ctx, otpVerifyPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
