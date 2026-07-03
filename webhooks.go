package silon

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SignatureHeader is the HTTP header carrying the webhook signature on
// every delivery to a subscribed endpoint.
const SignatureHeader = "Silon-Signature"

// DefaultWebhookTolerance is the maximum accepted clock skew, in seconds,
// between the signed timestamp and now.
const DefaultWebhookTolerance = 300

// webhookDigest computes hex(HMAC-SHA256(secret, "<ts>." + payload)).
func webhookDigest(secret string, ts int64, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// parseSignatureHeader extracts the t= timestamp and v1= signature from a
// "t=<unix_ts>,v1=<hex hmac>" header. ok is false when either is missing
// or malformed.
func parseSignatureHeader(header string) (ts int64, sig string, ok bool) {
	tsSet := false
	for part := range strings.SplitSeq(header, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch key {
		case "t":
			if isDigits(value) {
				if n, err := strconv.ParseInt(value, 10, 64); err == nil {
					ts = n
					tsSet = true
				}
			}
		case "v1":
			sig = value
		}
	}
	return ts, sig, tsSet && sig != ""
}

// SignWebhookPayload produces a valid Silon-Signature value — useful in
// tests and mocks. When ts <= 0, the current time is used.
func SignWebhookPayload(secret string, payload []byte, ts int64) string {
	if ts <= 0 {
		ts = time.Now().Unix()
	}
	return fmt.Sprintf("t=%d,v1=%s", ts, webhookDigest(secret, ts, payload))
}

// VerifyWebhookSignature reports whether header is a valid
// Silon-Signature for payload under the endpoint's whsec_ secret and
// within tolerance seconds of now. tolerance <= 0 skips the freshness
// check. The comparison is constant-time; a malformed header returns
// false.
func VerifyWebhookSignature(payload []byte, header, secret string, tolerance int) bool {
	ts, sig, ok := parseSignatureHeader(header)
	if !ok {
		return false
	}
	if tolerance > 0 {
		skew := time.Now().Unix() - ts
		if skew < 0 {
			skew = -skew
		}
		if skew > int64(tolerance) {
			return false
		}
	}
	expected := webhookDigest(secret, ts, payload)
	return hmac.Equal([]byte(expected), []byte(sig))
}

// ConstructWebhookEvent verifies the signature and parses payload into an
// *Event. It returns a *WebhookSignatureVerificationError when the
// signature is missing, stale, or does not match, and the base *Error when
// the payload is not valid JSON. Typical usage in an http.Handler:
//
//	body, _ := io.ReadAll(r.Body) // raw bytes, not parsed JSON
//	event, err := silon.ConstructWebhookEvent(
//		body, r.Header.Get(silon.SignatureHeader),
//		os.Getenv("SILON_WEBHOOK_SECRET"), silon.DefaultWebhookTolerance,
//	)
func ConstructWebhookEvent(payload []byte, header, secret string, tolerance int) (*Event, error) {
	if !VerifyWebhookSignature(payload, header, secret, tolerance) {
		return nil, &WebhookSignatureVerificationError{
			Message: "Webhook signature verification failed. Check that you are using the " +
				"endpoint's whsec_ secret and the raw (unparsed) request body.",
		}
	}
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, &Error{Message: "Could not parse webhook payload as JSON: " + err.Error()}
	}
	return &event, nil
}
