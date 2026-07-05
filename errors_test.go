package silon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

var standard400 = map[string]any{
	"type": "validation_error",
	"errors": []any{
		map[string]any{"code": "required", "detail": "This field is required.", "attr": "channel"},
		map[string]any{"code": "invalid", "detail": "Enter a valid value.", "attr": "to"},
	},
}

var inline404 = map[string]any{
	"type":   "https://acme.silon.tech/docs/errors/not-found",
	"title":  "Not Found",
	"status": 404,
	"detail": "No message with that id.",
	"field":  nil,
}

// retrieveErr performs a generic GET against the mock and returns its error.
func retrieveErr(t *testing.T, m *mockAPI) error {
	t.Helper()
	c := newTestClient(t, m)
	_, err := c.Messages.Retrieve(t.Context(), "evt-1")
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

func asAPIError(t *testing.T, err error) *APIError {
	t.Helper()
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T (%v)", err, err)
	}
	return apiErr
}

func TestStandardShapeParsed(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(400, standard400)))
	apiErr := asAPIError(t, retrieveErr(t, m))

	if apiErr.StatusCode != 400 || !IsBadRequest(apiErr) {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	if apiErr.ErrorType != "validation_error" {
		t.Errorf("ErrorType = %q", apiErr.ErrorType)
	}
	if len(apiErr.Errors) != 2 {
		t.Fatalf("len(Errors) = %d", len(apiErr.Errors))
	}
	if apiErr.Errors[0].Code != "required" || *apiErr.Errors[0].Attr != "channel" {
		t.Errorf("Errors[0] = %+v", apiErr.Errors[0])
	}
	if apiErr.Errors[1].Detail != "Enter a valid value." {
		t.Errorf("Errors[1] = %+v", apiErr.Errors[1])
	}
	if got := apiErr.Error(); !strings.Contains(got, "channel: This field is required.") {
		t.Errorf("Error() = %q", got)
	}
	var body map[string]any
	if err := json.Unmarshal(apiErr.Body, &body); err != nil {
		t.Fatalf("Body is not JSON: %v", err)
	}
	var want map[string]any
	raw, _ := json.Marshal(standard400)
	json.Unmarshal(raw, &want)
	if !reflect.DeepEqual(body, want) {
		t.Errorf("Body = %v, want %v", body, want)
	}
}

func TestInlineProblemShapeParsed(t *testing.T) {
	m := newMockAPI(t, always(jsonStubH(404, inline404,
		map[string]string{"Content-Type": "application/problem+json"})))
	apiErr := asAPIError(t, retrieveErr(t, m))

	if !IsNotFound(apiErr) {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.ErrorType != inline404["type"] {
		t.Errorf("ErrorType = %q", apiErr.ErrorType)
	}
	if len(apiErr.Errors) != 1 {
		t.Fatalf("len(Errors) = %d", len(apiErr.Errors))
	}
	if apiErr.Errors[0].Code != "not-found" { // slug pulled from the type URL
		t.Errorf("Code = %q", apiErr.Errors[0].Code)
	}
	if apiErr.Errors[0].Detail != "No message with that id." {
		t.Errorf("Detail = %q", apiErr.Errors[0].Detail)
	}
	if apiErr.Errors[0].Attr != nil {
		t.Errorf("Attr = %v, want nil", *apiErr.Errors[0].Attr)
	}
	if apiErr.Error() != "No message with that id." {
		t.Errorf("Error() = %q", apiErr.Error())
	}
}

func TestInlineProblemFieldBecomesAttr(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(422, map[string]any{
		"type":   "https://acme.silon.tech/docs/errors/unknown-channel",
		"title":  "Unknown channel",
		"status": 422,
		"detail": "Sending is not supported for channel='banana'.",
		"field":  "channel",
	})))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if !IsUnprocessableEntity(apiErr) {
		t.Errorf("StatusCode = %d, want 422", apiErr.StatusCode)
	}
	if apiErr.Errors[0].Attr == nil || *apiErr.Errors[0].Attr != "channel" {
		t.Errorf("Attr = %v, want channel", apiErr.Errors[0].Attr)
	}
	if apiErr.Errors[0].Code != "unknown-channel" {
		t.Errorf("Code = %q", apiErr.Errors[0].Code)
	}
}

func TestStatusCodeMapping(t *testing.T) {
	cases := []struct {
		status    int
		predicate func(error) bool
		name      string
	}{
		{400, IsBadRequest, "IsBadRequest"},
		{401, IsAuthentication, "IsAuthentication"},
		{403, IsPermissionDenied, "IsPermissionDenied"},
		{404, IsNotFound, "IsNotFound"},
		{409, IsConflict, "IsConflict"},
		{410, IsGone, "IsGone"},
		{422, IsUnprocessableEntity, "IsUnprocessableEntity"},
		{429, IsRateLimit, "IsRateLimit"},
		{500, IsInternalServer, "IsInternalServer"},
		{503, IsInternalServer, "IsInternalServer"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", tc.status, tc.name), func(t *testing.T) {
			m := newMockAPI(t, always(jsonStub(tc.status, map[string]any{"type": "x", "errors": []any{}})))
			err := retrieveErr(t, m)
			apiErr := asAPIError(t, err)
			if apiErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tc.status)
			}
			if !tc.predicate(err) {
				t.Errorf("%s(err) = false, want true", tc.name)
			}
		})
	}
}

func TestUnmapped4xxIsGenericAPIError(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(418, map[string]any{})))
	err := retrieveErr(t, m)
	apiErr := asAPIError(t, err)
	if apiErr.StatusCode != 418 {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	for name, predicate := range map[string]func(error) bool{
		"IsBadRequest": IsBadRequest, "IsAuthentication": IsAuthentication,
		"IsPermissionDenied": IsPermissionDenied, "IsNotFound": IsNotFound,
		"IsConflict": IsConflict, "IsGone": IsGone,
		"IsUnprocessableEntity": IsUnprocessableEntity, "IsRateLimit": IsRateLimit,
		"IsInternalServer": IsInternalServer,
	} {
		if predicate(err) {
			t.Errorf("%s(418 err) = true, want false", name)
		}
	}
}

func TestNonJSONBodyFallsBackToHTTPMessage(t *testing.T) {
	m := newMockAPI(t, always(rawStub(502, "Bad Gateway (apache)", map[string]string{"Content-Type": "text/plain"})))
	err := retrieveErr(t, m)
	apiErr := asAPIError(t, err)
	if !IsInternalServer(err) {
		t.Error("IsInternalServer = false")
	}
	if !strings.Contains(apiErr.Error(), "HTTP 502") {
		t.Errorf("Error() = %q", apiErr.Error())
	}
	if apiErr.Body != nil {
		t.Errorf("Body = %s, want nil", apiErr.Body)
	}
}

func TestRequestIDCaptured(t *testing.T) {
	m := newMockAPI(t, always(jsonStubH(401, map[string]any{},
		map[string]string{"X-Request-Id": "req_abc123"})))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if apiErr.RequestID != "req_abc123" {
		t.Errorf("RequestID = %q", apiErr.RequestID)
	}
}

func TestRetryAfterDeltaSeconds(t *testing.T) {
	m := newMockAPI(t, always(jsonStubH(429, map[string]any{},
		map[string]string{"Retry-After": "17"})))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if apiErr.RetryAfter == nil || *apiErr.RetryAfter != 17.0 {
		t.Errorf("RetryAfter = %v, want 17", apiErr.RetryAfter)
	}
}

func TestRetryAfterHTTPDate(t *testing.T) {
	when := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	m := newMockAPI(t, always(jsonStubH(429, map[string]any{},
		map[string]string{"Retry-After": when})))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if apiErr.RetryAfter == nil {
		t.Fatal("RetryAfter = nil")
	}
	if *apiErr.RetryAfter <= 0 || *apiErr.RetryAfter > 10.5 {
		t.Errorf("RetryAfter = %v, want in (0, 10.5]", *apiErr.RetryAfter)
	}
}

func TestRateLimitResetEpoch(t *testing.T) {
	reset := fmt.Sprint(time.Now().Unix() + 7)
	m := newMockAPI(t, always(jsonStubH(429, map[string]any{},
		map[string]string{"RateLimit-Reset": reset})))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if apiErr.RetryAfter == nil {
		t.Fatal("RetryAfter = nil")
	}
	if *apiErr.RetryAfter <= 0 || *apiErr.RetryAfter > 7.5 {
		t.Errorf("RetryAfter = %v, want in (0, 7.5]", *apiErr.RetryAfter)
	}
}

func TestRateLimitWithoutHeaders(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(429, map[string]any{})))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if apiErr.RetryAfter != nil {
		t.Errorf("RetryAfter = %v, want nil", *apiErr.RetryAfter)
	}
}

func TestRetryAfterOnlyOn429(t *testing.T) {
	m := newMockAPI(t, always(jsonStubH(400, map[string]any{},
		map[string]string{"Retry-After": "17"})))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if apiErr.RetryAfter != nil {
		t.Errorf("RetryAfter = %v on a 400, want nil", *apiErr.RetryAfter)
	}
}

func TestRetryableReadFromStandardBody(t *testing.T) {
	// A 429 standard body advertises retryable:true; read verbatim.
	m := newMockAPI(t, always(jsonStub(429, map[string]any{
		"type":      "throttled",
		"errors":    []any{map[string]any{"code": "throttled", "detail": "Slow down.", "attr": nil}},
		"retryable": true,
	})))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if apiErr.Retryable == nil || !*apiErr.Retryable {
		t.Errorf("Retryable = %v, want true", apiErr.Retryable)
	}
}

func TestRetryableFalseNotRecomputedFromStatus(t *testing.T) {
	// A 409 with retryable:false must stay false — never recomputed from
	// the status code (an idempotency-conflict 409 is not retryable).
	m := newMockAPI(t, always(jsonStub(409, map[string]any{
		"type":      "https://acme.silon.tech/docs/errors/idempotency-conflict",
		"title":     "Conflict",
		"status":    409,
		"detail":    "A different request already used this Idempotency-Key.",
		"retryable": false,
		"field":     nil,
	})))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if apiErr.Retryable == nil {
		t.Fatal("Retryable = nil, want a value")
	}
	if *apiErr.Retryable {
		t.Error("Retryable = true, want false (must be read verbatim, not recomputed)")
	}
}

func TestRetryableAbsentIsNil(t *testing.T) {
	// A legacy / non-v1 body omitting "retryable" leaves it nil.
	m := newMockAPI(t, always(jsonStub(400, standard400)))
	apiErr := asAPIError(t, retrieveErr(t, m))
	if apiErr.Retryable != nil {
		t.Errorf("Retryable = %v, want nil when the body omits the field", *apiErr.Retryable)
	}
}

func TestPredicatesRejectNonAPIErrors(t *testing.T) {
	for _, err := range []error{
		errors.New("plain"),
		&Error{Message: "config"},
		&ConnectionError{msg: "conn"},
		nil,
	} {
		if IsNotFound(err) || IsRateLimit(err) || IsInternalServer(err) {
			t.Errorf("predicate matched non-API error %v", err)
		}
	}
}
