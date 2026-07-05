package silon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Error is the base SDK error, used for configuration, client-side
// validation, and response-parse failures. API failures use *APIError and
// transport failures *ConnectionError.
type Error struct {
	Message string
}

func (e *Error) Error() string { return e.Message }

// ConnectionError means the request never produced an HTTP response
// (DNS, TLS, socket, timeout...). The underlying cause is available via
// errors.Unwrap / errors.Is / errors.As.
type ConnectionError struct {
	// Timeout is true when the failure was a timeout.
	Timeout bool

	msg string
	err error
}

func (e *ConnectionError) Error() string { return e.msg }

// Unwrap returns the underlying transport error.
func (e *ConnectionError) Unwrap() error { return e.err }

// WebhookSignatureVerificationError is returned by ConstructWebhookEvent
// when a payload fails Silon-Signature verification.
type WebhookSignatureVerificationError struct {
	Message string
}

func (e *WebhookSignatureVerificationError) Error() string { return e.Message }

// ErrorDetail is one normalized error entry from an API error response.
type ErrorDetail struct {
	// Code is the machine-readable error code (e.g. "required").
	Code string `json:"code"`
	// Detail is the human-readable description.
	Detail string `json:"detail"`
	// Attr is the request field the error applies to, when field-specific.
	Attr *string `json:"attr"`
}

// APIError is a non-2xx API response, normalized from both error body
// shapes the Silon API produces (standard {"type", "errors": [...]} and
// RFC 9457-style inline problems). Retrieve it with errors.As, or use the
// Is* predicate helpers (IsNotFound, IsRateLimit, ...).
type APIError struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// RequestID is the X-Request-Id response header, when present.
	RequestID string
	// ErrorType is the body's "type" discriminator (a slug for standard
	// errors, a documentation URL for inline problems).
	ErrorType string
	// Errors holds the normalized error entries.
	Errors []ErrorDetail
	// Body is the raw JSON error body; nil when the body was not valid
	// JSON. Useful for shapes carrying extra keys (e.g. the OTP-verify
	// failure's remaining_attempts).
	Body json.RawMessage
	// RetryAfter is the advertised backoff in seconds, parsed from the
	// Retry-After / RateLimit-Reset headers. Set on 429 responses only.
	RetryAfter *float64
	// Retryable mirrors the error body's top-level "retryable" bool: true
	// iff retrying the SAME request could ever succeed (HTTP 429, 5xx, or an
	// in-flight idempotency twin), false for every other 4xx (validation,
	// auth, permission, not-found, conflict, gone). It is read verbatim from
	// the body — never recomputed from the status code — and is nil when a
	// legacy / non-v1 error body omits the field.
	Retryable *bool
	// Message is the human-readable summary: "attr: detail" of the first
	// error, else its detail, else "HTTP <code>: <reason>".
	Message string
}

func (e *APIError) Error() string { return e.Message }

func statusIs(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == code
}

// IsBadRequest reports whether err is an *APIError with HTTP status 400.
func IsBadRequest(err error) bool { return statusIs(err, http.StatusBadRequest) }

// IsAuthentication reports whether err is an *APIError with HTTP status 401.
func IsAuthentication(err error) bool { return statusIs(err, http.StatusUnauthorized) }

// IsPermissionDenied reports whether err is an *APIError with HTTP status 403.
func IsPermissionDenied(err error) bool { return statusIs(err, http.StatusForbidden) }

// IsNotFound reports whether err is an *APIError with HTTP status 404.
func IsNotFound(err error) bool { return statusIs(err, http.StatusNotFound) }

// IsConflict reports whether err is an *APIError with HTTP status 409.
func IsConflict(err error) bool { return statusIs(err, http.StatusConflict) }

// IsGone reports whether err is an *APIError with HTTP status 410.
func IsGone(err error) bool { return statusIs(err, http.StatusGone) }

// IsUnprocessableEntity reports whether err is an *APIError with HTTP status 422.
func IsUnprocessableEntity(err error) bool { return statusIs(err, http.StatusUnprocessableEntity) }

// IsRateLimit reports whether err is an *APIError with HTTP status 429.
// Check the error's RetryAfter for the advertised backoff.
func IsRateLimit(err error) bool { return statusIs(err, http.StatusTooManyRequests) }

// IsInternalServer reports whether err is an *APIError with HTTP status >= 500.
func IsInternalServer(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode >= 500
}

// slugFromType turns "https://silon.tech/docs/errors/not-found" into
// "not-found"; values without a "/" pass through unchanged.
func slugFromType(errorType string) string {
	if strings.Contains(errorType, "/") {
		trimmed := strings.TrimRight(errorType, "/")
		if i := strings.LastIndex(trimmed, "/"); i >= 0 {
			return trimmed[i+1:]
		}
		return trimmed
	}
	return errorType
}

func jsonString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func optString(v any) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

// newAPIError builds an *APIError from a non-2xx response, normalizing
// both error body shapes per SPEC 2.
func newAPIError(resp *http.Response, body []byte) *APIError {
	var parsed any
	var raw json.RawMessage
	if len(body) > 0 && json.Unmarshal(body, &parsed) == nil {
		raw = json.RawMessage(append([]byte(nil), body...))
	} else {
		parsed = nil
	}

	var errorType string
	var details []ErrorDetail
	var retryable *bool
	message := ""

	if obj, ok := parsed.(map[string]any); ok {
		if t, ok := obj["type"].(string); ok {
			errorType = t
		}
		// Top-level "retryable" is present on both v1 body shapes; read it
		// verbatim (never recomputed from the status code) and leave it nil
		// when a legacy / non-v1 body omits it.
		if rb, ok := obj["retryable"].(bool); ok {
			retryable = &rb
		}
		if rawErrors, ok := obj["errors"].([]any); ok {
			// Standard DRF shape: {"type": ..., "errors": [{code, detail, attr}]}
			for _, entry := range rawErrors {
				if m, ok := entry.(map[string]any); ok {
					details = append(details, ErrorDetail{
						Code:   jsonString(m["code"]),
						Detail: jsonString(m["detail"]),
						Attr:   optString(m["attr"]),
					})
				}
			}
			if len(details) > 0 {
				first := details[0]
				if first.Attr != nil && *first.Attr != "" {
					message = *first.Attr + ": " + first.Detail
				} else {
					message = first.Detail
				}
			}
		} else if _, ok := obj["detail"]; ok {
			// Inline problem shape: {"type": url, "title", "status", "detail", "field"}
			detail := jsonString(obj["detail"])
			code := slugFromType(errorType)
			if code == "" {
				code = jsonString(obj["title"])
			}
			details = append(details, ErrorDetail{
				Code:   code,
				Detail: detail,
				Attr:   optString(obj["field"]),
			})
			message = detail
		}
	}

	if message == "" {
		reason := http.StatusText(resp.StatusCode)
		if reason == "" {
			reason = string(body)
			if len(reason) > 200 {
				reason = reason[:200]
			}
		}
		message = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, reason)
	}

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-Id"),
		ErrorType:  errorType,
		Errors:     details,
		Retryable:  retryable,
		Body:       raw,
		Message:    message,
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		apiErr.RetryAfter = parseRetryAfter(resp.Header)
	}
	return apiErr
}
