package silon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// Version is the SDK release version, reported in the User-Agent header.
const Version = "0.1.0"

// DefaultTimeout is the request timeout applied to the internally created
// HTTP client when WithTimeout / WithHTTPClient are not used.
const DefaultTimeout = 30 * time.Second

// DefaultMaxRetries is how many times a failed retryable request is retried
// when WithMaxRetries is not used.
const DefaultMaxRetries = 2

// Environment variables consulted by NewClient.
const (
	envAPIKey    = "SILON_API_KEY"
	envBaseURL   = "SILON_BASE_URL"
	envWorkspace = "SILON_WORKSPACE"
)

type clientConfig struct {
	apiKey         string
	workspace      string
	baseURL        string
	timeout        time.Duration
	maxRetries     int
	defaultHeaders map[string]string
	httpClient     *http.Client
}

// Option configures a Client created by NewClient.
type Option func(*clientConfig)

// WithAPIKey sets the API key. Falls back to the SILON_API_KEY environment
// variable when not provided.
func WithAPIKey(key string) Option {
	return func(c *clientConfig) { c.apiKey = key }
}

// WithWorkspace derives the base URL from a workspace slug:
// https://<workspace>.silon.tech. An explicit base URL (WithBaseURL or
// SILON_BASE_URL) takes precedence.
func WithWorkspace(workspace string) Option {
	return func(c *clientConfig) { c.workspace = workspace }
}

// WithBaseURL sets the API base URL explicitly (e.g. an on-prem
// deployment). A trailing slash is stripped.
func WithBaseURL(baseURL string) Option {
	return func(c *clientConfig) { c.baseURL = baseURL }
}

// WithTimeout sets the request timeout on the internally created HTTP
// client (default 30s). Ignored when WithHTTPClient is supplied — the
// custom client's own Timeout governs then.
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) { c.timeout = d }
}

// WithMaxRetries sets how many times a failed retryable request is retried
// (default 2). Zero disables retries.
func WithMaxRetries(n int) Option {
	return func(c *clientConfig) { c.maxRetries = n }
}

// WithDefaultHeader adds a header sent on every request. May be repeated;
// later values for the same key win.
func WithDefaultHeader(key, value string) Option {
	return func(c *clientConfig) {
		if c.defaultHeaders == nil {
			c.defaultHeaders = map[string]string{}
		}
		c.defaultHeaders[key] = value
	}
}

// WithHTTPClient supplies a custom *http.Client — useful for TLS control
// (private CAs), proxies, or instrumentation. When set, the custom
// client's Timeout governs requests and WithTimeout is ignored.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *clientConfig) { c.httpClient = hc }
}

// Client is the Silon API client. Construct it with NewClient; the zero
// value is not usable. A Client is safe for concurrent use.
type Client struct {
	apiKey         string
	baseURL        string
	maxRetries     int
	defaultHeaders map[string]string
	httpClient     *http.Client

	// sleep pauses between retry attempts; injectable in tests. It must
	// return early with the context's error when ctx is cancelled.
	sleep func(ctx context.Context, d time.Duration) error

	// Messages sends messages on any channel and looks up delivery status.
	Messages *MessagesService

	// Broadcasts inspects audience fan-outs created by Messages.Send:
	// aggregate counts and per-recipient delivery rows.
	Broadcasts *BroadcastsService

	// OTP sends and verifies one-time passwords.
	OTP *OTPService

	// Clients manages CRM client profiles.
	Clients *ClientsService

	// ClientGroups manages CRM client groups (broadcast audiences).
	ClientGroups *ClientGroupsService

	// Bulk runs bulk (CSV) sends: batches, saved files (Bulk.Files), and
	// per-recipient rows (Bulk.Recipients).
	Bulk *BulkService

	// Events reads the event stream your webhook endpoints are fed from.
	Events *EventsService

	// WebhookEndpoints manages outbound webhook subscriptions.
	WebhookEndpoints *WebhookEndpointsService

	// Suppressions manages the workspace's do-not-contact list, enforced
	// on every send path.
	Suppressions *SuppressionsService

	// Reports runs activity reports (messages, channels, clients, users,
	// bulks, subscriptions, AWS usage) and provider balance lookups.
	Reports *ReportsService

	// WhatsAppTemplates lists approved WhatsApp templates.
	WhatsAppTemplates *WhatsAppTemplatesService

	// Templates manages slug-keyed message templates with an immutable
	// version spine.
	Templates *TemplatesService

	// Push registers mobile / web push devices and reads the legacy
	// native notification feeds.
	Push *PushService

	// Profile reads and updates the authenticated user's own profile.
	Profile *ProfileService

	// Auth signs up users.
	Auth *AuthService
}

// NewClient builds a Client from functional options with environment
// fallbacks. It fails fast (returning the base *Error) when no API key or
// base URL can be resolved.
//
// Resolution order:
//
//   - API key: WithAPIKey, else SILON_API_KEY. Required.
//   - Base URL: WithBaseURL, else SILON_BASE_URL, else WithWorkspace ->
//     https://<workspace>.silon.tech, else SILON_WORKSPACE (same
//     expansion). Required; trailing slash stripped.
func NewClient(opts ...Option) (*Client, error) {
	cfg := clientConfig{
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	apiKey := cfg.apiKey
	if apiKey == "" {
		apiKey = os.Getenv(envAPIKey)
	}
	if apiKey == "" {
		return nil, &Error{Message: "No API key provided. Pass silon.WithAPIKey(...) or set the " +
			"SILON_API_KEY environment variable. Create a key in the dashboard under Settings > API keys."}
	}

	baseURL := cfg.baseURL
	if baseURL == "" {
		baseURL = os.Getenv(envBaseURL)
	}
	if baseURL == "" {
		workspace := cfg.workspace
		if workspace == "" {
			workspace = os.Getenv(envWorkspace)
		}
		if workspace != "" {
			baseURL = "https://" + workspace + ".silon.tech"
		}
	}
	if baseURL == "" {
		return nil, &Error{Message: "No base URL. Pass silon.WithWorkspace(\"<your-workspace>\") " +
			"(=> https://<workspace>.silon.tech), silon.WithBaseURL(...), or set SILON_WORKSPACE / SILON_BASE_URL."}
	}

	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.timeout}
	}

	headers := make(map[string]string, len(cfg.defaultHeaders))
	for k, v := range cfg.defaultHeaders {
		headers[k] = v
	}

	c := &Client{
		apiKey:         apiKey,
		baseURL:        strings.TrimRight(baseURL, "/"),
		maxRetries:     cfg.maxRetries,
		defaultHeaders: headers,
		httpClient:     httpClient,
		sleep:          sleepContext,
	}
	c.Messages = &MessagesService{client: c}
	c.Broadcasts = &BroadcastsService{client: c}
	c.OTP = &OTPService{client: c}
	c.Clients = &ClientsService{client: c}
	c.ClientGroups = &ClientGroupsService{client: c}
	c.Bulk = &BulkService{
		client:     c,
		Files:      &BulkFilesService{client: c},
		Recipients: &BulkRecipientsService{client: c},
	}
	c.Events = &EventsService{client: c}
	c.WebhookEndpoints = &WebhookEndpointsService{client: c}
	c.Suppressions = &SuppressionsService{client: c}
	c.Reports = &ReportsService{client: c}
	c.WhatsAppTemplates = &WhatsAppTemplatesService{client: c}
	c.Templates = &TemplatesService{client: c}
	c.Push = &PushService{client: c}
	c.Profile = &ProfileService{client: c}
	c.Auth = &AuthService{client: c}
	return c, nil
}

// BaseURL returns the resolved API base URL (no trailing slash).
func (c *Client) BaseURL() string { return c.baseURL }

func userAgent() string {
	return fmt.Sprintf("silon-go/%s go/%s", Version, strings.TrimPrefix(runtime.Version(), "go"))
}

// sleepContext waits for d, returning early with ctx.Err() when the
// context is cancelled first. It is the default retry sleeper.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
