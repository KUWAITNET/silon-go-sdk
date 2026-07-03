package silon

import (
	"context"
	"net/url"
)

const (
	subscribeAndroidPath = "/api/v1/subscribe/android/"
	subscribeIOSPath     = "/api/v1/subscribe/ios/"
	pushClientPath       = "/api/v1/push/client/"
	pushReadPath         = "/api/v1/push/read/"
	webPushClientPath    = "/api/v1/webpush/client/"
)

// PushSubscribeResult is the success body of the device subscribe
// endpoints.
type PushSubscribeResult struct {
	// Success is always 1 on success.
	Success int `json:"success"`
}

// PushMarkReadResult is the body of POST /api/v1/push/read/.
type PushMarkReadResult struct {
	// AffectedRows is the number of notifications transitioned to read.
	AffectedRows int `json:"affected_rows"`
}

// PushNotification is one native push notification row from the legacy
// list endpoints.
type PushNotification struct {
	Message string `json:"message"`
	Subject string `json:"subject"`

	// Date is when the notification was sent. The legacy feed returns
	// date-only values, so it stays a string.
	Date string `json:"date"`
}

// PushClientDevices is the echo body of POST /api/v1/push/client/.
type PushClientDevices struct {
	ClientID   string `json:"client_id"`
	Slug       string `json:"slug"`
	DeviceID   string `json:"device_id,omitempty"`
	DeviceType string `json:"device_type"`

	KeepDevices *bool `json:"keep_devices,omitempty"`
}

// WebPushSubscription is the echo body of the legacy
// POST /api/v1/webpush/client/.
type WebPushSubscription struct {
	ClientID         string `json:"client_id"`
	Slug             string `json:"slug"`
	SubscriptionInfo string `json:"subscription_info,omitempty"`
}

// PushPlatform selects which native notification feed
// PushService.ListNotifications reads.
type PushPlatform string

const (
	// PushPlatformAndroid reads the Android feed.
	PushPlatformAndroid PushPlatform = "android"

	// PushPlatformIOS reads the iOS feed.
	PushPlatformIOS PushPlatform = "ios"

	// PushPlatformCombined (the zero value) reads the merged feed.
	PushPlatformCombined PushPlatform = ""
)

// notificationsPath maps a platform to its legacy list endpoint,
// erroring client-side on an unknown platform.
func notificationsPath(slug string, platform PushPlatform) (string, error) {
	escaped := url.PathEscape(slug)
	switch platform {
	case PushPlatformAndroid:
		return "/api/v1/push/android/" + escaped + "/", nil
	case PushPlatformIOS:
		return "/api/v1/push/ios/" + escaped + "/", nil
	case PushPlatformCombined:
		return "/api/v1/push/list/" + escaped + "/", nil
	default:
		return "", &Error{Message: "platform must be PushPlatformAndroid, PushPlatformIOS or " +
			"PushPlatformCombined (the combined list)."}
	}
}

// PushSubscribeAndroidParams are the parameters for
// PushService.SubscribeAndroid. Nil fields are omitted from the request
// JSON.
type PushSubscribeAndroidParams struct {
	// Slug is required: the application slug to register the device
	// under.
	Slug string

	// Token is the FCM push token issued by the platform.
	Token *string
}

// PushSubscribeIOSParams are the parameters for
// PushService.SubscribeIOS. Nil fields are omitted from the request
// JSON.
type PushSubscribeIOSParams struct {
	// Slug is required: the application slug to register the device
	// under.
	Slug string

	// Token is the APNs push token issued by the platform.
	Token *string

	// Environment is the APNs environment the token was minted in,
	// driven by the iOS build's aps-environment entitlement.
	Environment *string
}

// PushUpsertDevicesParams are the parameters for
// PushService.UpsertDevices. Nil fields are omitted from the request
// JSON.
type PushUpsertDevicesParams struct {
	// ClientID is required: the client whose push devices to
	// register/prune.
	ClientID string

	// Slug is required: the application slug the device belongs to.
	Slug string

	// DeviceType is required: "ios", "android" or "huawei".
	DeviceType string

	// DeviceID is the device push token to register for the given
	// platform. Omit to prune only.
	DeviceID *string

	// KeepDevices false (the server default) removes the client's other
	// tokens for this platform/app.
	KeepDevices *bool
}

// PushSubscribeWebParams are the parameters for the deprecated
// PushService.SubscribeWeb. Nil fields are omitted from the request
// JSON.
type PushSubscribeWebParams struct {
	// ClientID is required: the client to attach the browser
	// subscription to.
	ClientID string

	// Slug is required: the Web Push widget slug the visitor subscribed
	// through.
	Slug string

	// SubscriptionInfo is the browser PushSubscription JSON (endpoint +
	// p256dh/auth keys), as a string.
	SubscriptionInfo *string
}

// PushService registers mobile / web push devices and reads the legacy
// native notification feeds. Access it via Client.Push.
type PushService struct {
	client *Client
}

// SubscribeAndroid registers an Android device token for an app
// (POST /api/v1/subscribe/android/).
func (s *PushService) SubscribeAndroid(ctx context.Context, params PushSubscribeAndroidParams) (*PushSubscribeResult, error) {
	body := map[string]any{"slug": params.Slug}
	if params.Token != nil {
		body["token"] = *params.Token
	}
	var out PushSubscribeResult
	if err := s.client.post(ctx, subscribeAndroidPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SubscribeIOS registers an iOS device token for an app
// (POST /api/v1/subscribe/ios/), optionally pinning the APNs
// environment.
func (s *PushService) SubscribeIOS(ctx context.Context, params PushSubscribeIOSParams) (*PushSubscribeResult, error) {
	body := map[string]any{"slug": params.Slug}
	if params.Token != nil {
		body["token"] = *params.Token
	}
	if params.Environment != nil {
		body["environment"] = *params.Environment
	}
	var out PushSubscribeResult
	if err := s.client.post(ctx, subscribeIOSPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpsertDevices registers (or prunes, with KeepDevices false) a
// client's push devices (POST /api/v1/push/client/).
func (s *PushService) UpsertDevices(ctx context.Context, params PushUpsertDevicesParams) (*PushClientDevices, error) {
	body := map[string]any{
		"client_id":   params.ClientID,
		"slug":        params.Slug,
		"device_type": params.DeviceType,
	}
	if params.DeviceID != nil {
		body["device_id"] = *params.DeviceID
	}
	if params.KeepDevices != nil {
		body["keep_devices"] = *params.KeepDevices
	}
	var out PushClientDevices
	if err := s.client.post(ctx, pushClientPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MarkRead marks an app's unread native push notifications as read
// (POST /api/v1/push/read/).
func (s *PushService) MarkRead(ctx context.Context, slug string) (*PushMarkReadResult, error) {
	var out PushMarkReadResult
	if err := s.client.post(ctx, pushReadPath, map[string]any{"slug": slug}, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListNotifications lists the native push notifications sent to an app
// (GET /api/v1/push/android/{slug}/, /api/v1/push/ios/{slug}/ or, for
// PushPlatformCombined, /api/v1/push/list/{slug}/ — the API returns a
// bare JSON array). An unknown platform returns a client-side *Error
// without any request being made.
//
// Deprecated: the native push notification list endpoints are legacy;
// use the Events API / webhook endpoints for delivery visibility
// instead.
func (s *PushService) ListNotifications(ctx context.Context, slug string, platform PushPlatform) ([]PushNotification, error) {
	path, err := notificationsPath(slug, platform)
	if err != nil {
		return nil, err
	}
	var out []PushNotification
	if err := s.client.get(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SubscribeWeb registers a browser subscription for a client
// (POST /api/v1/webpush/client/).
//
// Deprecated: POST /api/v1/webpush/client/ is a legacy endpoint;
// register web push subscriptions through the widget instead.
func (s *PushService) SubscribeWeb(ctx context.Context, params PushSubscribeWebParams) (*WebPushSubscription, error) {
	body := map[string]any{
		"client_id": params.ClientID,
		"slug":      params.Slug,
	}
	if params.SubscriptionInfo != nil {
		body["subscription_info"] = *params.SubscriptionInfo
	}
	var out WebPushSubscription
	if err := s.client.post(ctx, webPushClientPath, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
