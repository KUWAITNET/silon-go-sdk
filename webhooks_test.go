package silon

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const webhookSecret = "whsec_test_secret"

var webhookPayload = mustJSON(map[string]any{
	"id":          "evt_01J1",
	"object":      "event",
	"type":        "broadcast.completed",
	"api_version": "2026-06-28",
	"created":     "2026-07-02T10:00:00Z",
	"data": map[string]any{
		"id": "br_1", "object": "broadcast",
		"target_count": 100, "sent": 97, "failed": 3,
	},
})

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func TestSignVerifyRoundtrip(t *testing.T) {
	header := SignWebhookPayload(webhookSecret, webhookPayload, 0)
	if !VerifyWebhookSignature(webhookPayload, header, webhookSecret, DefaultWebhookTolerance) {
		t.Error("roundtrip verification failed")
	}
}

func TestHeaderFormat(t *testing.T) {
	header := SignWebhookPayload(webhookSecret, []byte("x"), 1_700_000_000)
	if !strings.HasPrefix(header, "t=1700000000,v1=") {
		t.Errorf("header = %q", header)
	}
	hexPart := strings.TrimPrefix(header, "t=1700000000,v1=")
	if len(hexPart) != 64 { // hex-encoded SHA-256
		t.Errorf("v1 length = %d, want 64", len(hexPart))
	}
}

func TestWrongSecretRejected(t *testing.T) {
	header := SignWebhookPayload("whsec_right", []byte("x"), 0)
	if VerifyWebhookSignature([]byte("x"), header, "whsec_wrong", DefaultWebhookTolerance) {
		t.Error("wrong secret accepted")
	}
}

func TestTamperedPayloadRejected(t *testing.T) {
	header := SignWebhookPayload(webhookSecret, webhookPayload, 0)
	tampered := bytes.ReplaceAll(webhookPayload, []byte(`"sent":97`), []byte(`"sent":100`))
	if bytes.Equal(tampered, webhookPayload) {
		t.Fatal("tampering had no effect; fixture drifted")
	}
	if VerifyWebhookSignature(tampered, header, webhookSecret, DefaultWebhookTolerance) {
		t.Error("tampered payload accepted")
	}
}

func TestStaleTimestampRejected(t *testing.T) {
	stale := time.Now().Unix() - 3600
	header := SignWebhookPayload(webhookSecret, webhookPayload, stale)
	if VerifyWebhookSignature(webhookPayload, header, webhookSecret, DefaultWebhookTolerance) {
		t.Error("hour-old signature accepted with 300s tolerance")
	}
}

func TestToleranceZeroSkipsFreshnessCheck(t *testing.T) {
	stale := time.Now().Unix() - 3600
	header := SignWebhookPayload(webhookSecret, webhookPayload, stale)
	if !VerifyWebhookSignature(webhookPayload, header, webhookSecret, 0) {
		t.Error("tolerance 0 must skip the freshness check")
	}
}

func TestMalformedHeadersRejected(t *testing.T) {
	for _, header := range []string{"", "garbage", "t=notdigits,v1=aa", "t=123", "v1=aa"} {
		t.Run(header, func(t *testing.T) {
			if VerifyWebhookSignature(webhookPayload, header, webhookSecret, DefaultWebhookTolerance) {
				t.Errorf("malformed header %q accepted", header)
			}
		})
	}
}

func TestConstructWebhookEventReturnsTypedEvent(t *testing.T) {
	header := SignWebhookPayload(webhookSecret, webhookPayload, 0)
	event, err := ConstructWebhookEvent(webhookPayload, header, webhookSecret, DefaultWebhookTolerance)
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "broadcast.completed" || event.ID != "evt_01J1" {
		t.Errorf("event = %+v", event)
	}
	if event.Data.Sent == nil || *event.Data.Sent != 97 {
		t.Errorf("Data.Sent = %v", event.Data.Sent)
	}
	if event.Data.Failed == nil || *event.Data.Failed != 3 {
		t.Errorf("Data.Failed = %v", event.Data.Failed)
	}
	if event.Created == nil || !event.Created.Equal(time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("Created = %v", event.Created)
	}
}

func TestConstructWebhookEventRejectsBadSignature(t *testing.T) {
	header := SignWebhookPayload("whsec_other", webhookPayload, 0)
	_, err := ConstructWebhookEvent(webhookPayload, header, webhookSecret, DefaultWebhookTolerance)
	var sigErr *WebhookSignatureVerificationError
	if !errors.As(err, &sigErr) {
		t.Fatalf("want *WebhookSignatureVerificationError, got %T (%v)", err, err)
	}
	if !strings.Contains(sigErr.Error(), "verification failed") {
		t.Errorf("message = %q", sigErr.Error())
	}
}

func TestSignatureHeaderConstant(t *testing.T) {
	if SignatureHeader != "Silon-Signature" {
		t.Errorf("SignatureHeader = %q", SignatureHeader)
	}
}
