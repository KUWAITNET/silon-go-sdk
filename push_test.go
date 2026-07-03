package silon

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestPushSubscribeAndroid(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{"success": 1})))
	c := newTestClient(t, m)

	result, err := c.Push.SubscribeAndroid(t.Context(), PushSubscribeAndroidParams{
		Slug:  "consumer-app",
		Token: String("fcm-token"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success != 1 {
		t.Errorf("Success = %d", result.Success)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/subscribe/android/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"slug": "consumer-app", "token": "fcm-token"}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	if got := last.header.Get("Idempotency-Key"); got != "" {
		t.Errorf("subscribe must not send an Idempotency-Key, got %q", got)
	}
}

func TestPushSubscribeIOSWithEnvironment(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{"success": 1})))
	c := newTestClient(t, m)

	result, err := c.Push.SubscribeIOS(t.Context(), PushSubscribeIOSParams{
		Slug:        "consumer-app",
		Token:       String("apns-token"),
		Environment: String("prod"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success != 1 {
		t.Errorf("Success = %d", result.Success)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/subscribe/ios/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"slug":        "consumer-app",
		"token":       "apns-token",
		"environment": "prod",
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestPushUpsertDevices(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"client_id":    "cust_001",
		"slug":         "consumer-app",
		"device_id":    "d1",
		"device_type":  "android",
		"keep_devices": false,
	})))
	c := newTestClient(t, m)

	result, err := c.Push.UpsertDevices(t.Context(), PushUpsertDevicesParams{
		ClientID:    "cust_001",
		Slug:        "consumer-app",
		DeviceType:  "android",
		DeviceID:    String("d1"),
		KeepDevices: Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ClientID != "cust_001" || result.DeviceID != "d1" || result.DeviceType != "android" {
		t.Errorf("result = %+v", result)
	}
	if result.KeepDevices == nil || *result.KeepDevices {
		t.Errorf("KeepDevices = %v, want false", result.KeepDevices)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/push/client/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"client_id":    "cust_001",
		"slug":         "consumer-app",
		"device_type":  "android",
		"device_id":    "d1",
		"keep_devices": false,
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestPushMarkRead(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{"affected_rows": 2})))
	c := newTestClient(t, m)

	result, err := c.Push.MarkRead(t.Context(), "consumer-app")
	if err != nil {
		t.Fatal(err)
	}
	if result.AffectedRows != 2 {
		t.Errorf("AffectedRows = %d", result.AffectedRows)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/push/read/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"slug": "consumer-app"}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestPushListNotificationsPaths(t *testing.T) {
	notifications := []any{
		map[string]any{"message": "Hello", "subject": "Hi", "date": "2026-07-01"},
	}
	cases := []struct {
		name     string
		platform PushPlatform
		path     string
	}{
		{"android", PushPlatformAndroid, "/api/v1/push/android/consumer-app/"},
		{"ios", PushPlatformIOS, "/api/v1/push/ios/consumer-app/"},
		{"combined", PushPlatformCombined, "/api/v1/push/list/consumer-app/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMockAPI(t, always(jsonStub(200, notifications)))
			c := newTestClient(t, m)

			got, err := c.Push.ListNotifications(t.Context(), "consumer-app", tc.platform)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Subject != "Hi" || got[0].Message != "Hello" ||
				got[0].Date != "2026-07-01" {
				t.Errorf("notifications = %+v", got)
			}

			last := m.lastCall(t)
			if last.method != "GET" || last.path != tc.path {
				t.Errorf("%s %s, want GET %s", last.method, last.path, tc.path)
			}
		})
	}
}

func TestPushListNotificationsRejectsUnknownPlatform(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, []any{})))
	c := newTestClient(t, m)

	_, err := c.Push.ListNotifications(t.Context(), "consumer-app", PushPlatform("windows"))
	var baseErr *Error
	if !errors.As(err, &baseErr) {
		t.Fatalf("want a client-side *Error, got %v", err)
	}
	if !strings.Contains(baseErr.Message, "platform") {
		t.Errorf("message = %q, want it to mention 'platform'", baseErr.Message)
	}
	if m.callCount() != 0 {
		t.Errorf("mock API received %d calls, want 0", m.callCount())
	}
}

func TestPushSubscribeWeb(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"client_id":         "cust_001",
		"slug":              "widget",
		"subscription_info": "{}",
	})))
	c := newTestClient(t, m)

	result, err := c.Push.SubscribeWeb(t.Context(), PushSubscribeWebParams{
		ClientID:         "cust_001",
		Slug:             "widget",
		SubscriptionInfo: String("{}"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ClientID != "cust_001" || result.Slug != "widget" || result.SubscriptionInfo != "{}" {
		t.Errorf("result = %+v", result)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/webpush/client/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"client_id":         "cust_001",
		"slug":              "widget",
		"subscription_info": "{}",
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}
