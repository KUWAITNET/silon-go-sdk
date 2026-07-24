package silon

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// reportsBasePath is the prefix shared by every reports endpoint.
const reportsBasePath = "/api/v1/reports"

// Report is the common envelope returned by every POST
// /api/v1/reports/* endpoint.
//
// Row columns in ReportData vary per report (and per report type), so
// rows stay generic maps.
type Report struct {
	// ReportData is the page of report rows.
	ReportData []map[string]any `json:"report_data"`

	// TotalItems is the total number of matching rows across all pages.
	TotalItems int `json:"total_items"`

	// TotalPages is the number of pages at the tenant's configured page
	// size.
	TotalPages int `json:"total_pages"`

	// Page is the 1-based page index contained in this response.
	Page int `json:"page"`

	// ReportType echoes the requested report_type, on the reports that
	// take one.
	ReportType string `json:"report_type,omitempty"`
}

// ProviderBalance is the body of GET /api/v1/reports/balance/{slug}/.
type ProviderBalance struct {
	// Balance is the upstream provider balance (provider-specific
	// format; may be a number). Empty string when the account has no
	// balance lookup.
	Balance string `json:"balance"`
}

// ConversationsReport is the body of
// GET /api/v1/reports/conversations/ — support-desk metrics for Live
// Desk conversations.
//
// First-response / resolution / reply times, CSAT, and open /
// unassigned / unattended gauges, with an agent / channel / team /
// label breakdown. Totals and Rows stay generic maps (columns vary by
// GroupBy); PreviousTotals and Deltas are present only in compare mode.
type ConversationsReport struct {
	// GroupBy is the breakdown dimension: "agent", "channel", "team" or
	// "label".
	GroupBy string `json:"group_by"`

	// BusinessHours reports whether times were measured against business
	// hours only.
	BusinessHours bool `json:"business_hours"`

	// DateFrom / DateTo echo the requested report window, or nil when
	// unbounded.
	DateFrom *string `json:"date_from"`
	DateTo   *string `json:"date_to"`

	// Totals holds the aggregate metrics for the window. Columns vary by
	// GroupBy, so it stays a generic map.
	Totals map[string]any `json:"totals"`

	// Rows holds one entry per group (agent/channel/team/label). Columns
	// vary by GroupBy, so rows stay generic maps.
	Rows []map[string]any `json:"rows"`

	// PreviousTotals holds the prior-window aggregates, present only in
	// compare mode (nil otherwise).
	PreviousTotals map[string]any `json:"previous_totals,omitempty"`

	// Deltas holds the change versus the prior window, present only in
	// compare mode (nil otherwise).
	Deltas map[string]any `json:"deltas,omitempty"`
}

// ReportsService runs activity reports and provider balance lookups
// (/api/v1/reports/...). Access it via Client.Reports.
type ReportsService struct {
	client *Client
}

// MessagesReportParams are the parameters for ReportsService.Messages.
// Only ReportType is required; nil fields are omitted from the request
// JSON.
type MessagesReportParams struct {
	// ReportType is required: "phone" (SMS/TTS/WhatsApp), "email",
	// "mobile" (push) or "web" (web push).
	ReportType string

	// DateFrom / DateTo bound the report window ("YYYY-MM-DD").
	DateFrom *string
	DateTo   *string

	// Search filters rows by free text.
	Search *string

	// Device filters by device platform.
	Device []string

	// Status filters by delivery status.
	Status []string

	// Template filters by message template ids.
	Template []int

	// Source filters by message source.
	Source []string
}

func (p MessagesReportParams) body() map[string]any {
	body := map[string]any{"report_type": p.ReportType}
	if p.DateFrom != nil {
		body["date_from"] = *p.DateFrom
	}
	if p.DateTo != nil {
		body["date_to"] = *p.DateTo
	}
	if p.Search != nil {
		body["search"] = *p.Search
	}
	if p.Device != nil {
		body["device"] = p.Device
	}
	if p.Status != nil {
		body["status"] = p.Status
	}
	if p.Template != nil {
		body["template"] = p.Template
	}
	if p.Source != nil {
		body["source"] = p.Source
	}
	return body
}

// ChannelsReportParams are the parameters for ReportsService.Channels.
// All fields are optional; nil fields are omitted from the request JSON.
type ChannelsReportParams struct {
	// DateFrom / DateTo bound the report window ("YYYY-MM-DD").
	DateFrom *string
	DateTo   *string

	// ChannelName filters to the named channels.
	ChannelName []string
}

func (p ChannelsReportParams) body() map[string]any {
	body := map[string]any{}
	if p.DateFrom != nil {
		body["date_from"] = *p.DateFrom
	}
	if p.DateTo != nil {
		body["date_to"] = *p.DateTo
	}
	if p.ChannelName != nil {
		body["channel_name"] = p.ChannelName
	}
	return body
}

// ClientsReportParams are the parameters for ReportsService.Clients.
// All fields are optional; nil fields are omitted from the request JSON.
type ClientsReportParams struct {
	// DateFrom / DateTo bound the report window ("YYYY-MM-DD").
	DateFrom *string
	DateTo   *string

	// Search filters rows by free text.
	Search *string

	// Language filters by the client's default language.
	Language []string

	// Device filters by device platform.
	Device []string

	// WebSubscription filters by web push subscription state.
	WebSubscription *string
}

func (p ClientsReportParams) body() map[string]any {
	body := map[string]any{}
	if p.DateFrom != nil {
		body["date_from"] = *p.DateFrom
	}
	if p.DateTo != nil {
		body["date_to"] = *p.DateTo
	}
	if p.Search != nil {
		body["search"] = *p.Search
	}
	if p.Language != nil {
		body["language"] = p.Language
	}
	if p.Device != nil {
		body["device"] = p.Device
	}
	if p.WebSubscription != nil {
		body["web_subscription"] = *p.WebSubscription
	}
	return body
}

// UsersReportParams are the parameters for ReportsService.Users. All
// fields are optional; nil fields are omitted from the request JSON.
type UsersReportParams struct {
	// DateFrom / DateTo bound the report window ("YYYY-MM-DD").
	DateFrom *string
	DateTo   *string

	// Search filters rows by free text.
	Search *string
}

// BulksReportParams are the parameters for ReportsService.Bulks. All
// fields are optional; nil fields are omitted from the request JSON.
type BulksReportParams struct {
	// DateFrom / DateTo bound the report window ("YYYY-MM-DD").
	DateFrom *string
	DateTo   *string

	// Search filters rows by free text.
	Search *string
}

func dateRangeReportBody(dateFrom, dateTo, search *string) map[string]any {
	body := map[string]any{}
	if dateFrom != nil {
		body["date_from"] = *dateFrom
	}
	if dateTo != nil {
		body["date_to"] = *dateTo
	}
	if search != nil {
		body["search"] = *search
	}
	return body
}

// SpecificBulksReportParams are the parameters for
// ReportsService.SpecificBulks. Only BulksFile is required; nil fields
// are omitted from the request JSON.
type SpecificBulksReportParams struct {
	// BulksFile is required: the bulk batch id to report on.
	BulksFile int

	// DateFrom / DateTo bound the report window ("YYYY-MM-DD").
	DateFrom *string
	DateTo   *string

	// Search filters rows by free text.
	Search *string

	// MobileApp filters by push application slug.
	MobileApp []string

	// WebApp filters by web push widget slug.
	WebApp []string

	// Status filters by delivery status.
	Status []string
}

func (p SpecificBulksReportParams) body() map[string]any {
	body := map[string]any{"bulks_file": p.BulksFile}
	if p.DateFrom != nil {
		body["date_from"] = *p.DateFrom
	}
	if p.DateTo != nil {
		body["date_to"] = *p.DateTo
	}
	if p.Search != nil {
		body["search"] = *p.Search
	}
	if p.MobileApp != nil {
		body["mobile_app"] = p.MobileApp
	}
	if p.WebApp != nil {
		body["web_app"] = p.WebApp
	}
	if p.Status != nil {
		body["status"] = p.Status
	}
	return body
}

// SubscriptionsReportParams are the parameters for
// ReportsService.Subscriptions. Only ReportType is required; nil fields
// are omitted from the request JSON.
type SubscriptionsReportParams struct {
	// ReportType is required, e.g. "mobile" or "web".
	ReportType string

	// DateFrom / DateTo bound the report window ("YYYY-MM-DD").
	DateFrom *string
	DateTo   *string

	// Search filters rows by free text.
	Search *string

	// MobileApp filters by push application slug.
	MobileApp []string

	// WebApp filters by web push widget slug.
	WebApp []string

	// DeviceType filters by device platform.
	DeviceType []string
}

func (p SubscriptionsReportParams) body() map[string]any {
	body := map[string]any{"report_type": p.ReportType}
	if p.DateFrom != nil {
		body["date_from"] = *p.DateFrom
	}
	if p.DateTo != nil {
		body["date_to"] = *p.DateTo
	}
	if p.Search != nil {
		body["search"] = *p.Search
	}
	if p.MobileApp != nil {
		body["mobile_app"] = p.MobileApp
	}
	if p.WebApp != nil {
		body["web_app"] = p.WebApp
	}
	if p.DeviceType != nil {
		body["device_type"] = p.DeviceType
	}
	return body
}

// AWSUsageReportParams are the parameters for ReportsService.AWSUsage.
// Page, when non-nil, is sent as a query parameter (the request has no
// body).
type AWSUsageReportParams struct {
	// Page is the 1-based page to fetch.
	Page *int
}

// ConversationsReportParams are the parameters for
// ReportsService.Conversations. All fields are optional; nil fields are
// omitted from the query string (the request has no body).
type ConversationsReportParams struct {
	// DateFrom / DateTo bound the report window (ISO datetime).
	DateFrom *string
	DateTo   *string

	// GroupBy is the breakdown dimension: "agent", "channel", "team" or
	// "label".
	GroupBy *string

	// BusinessHours, when true, measures times against business hours
	// only.
	BusinessHours *bool

	// Channels filters to a comma-separated list of channel slugs.
	Channels *string

	// Compare, when true, includes prior-window totals and deltas.
	Compare *bool
}

func (p ConversationsReportParams) query() url.Values {
	q := url.Values{}
	if p.DateFrom != nil {
		q.Set("date_from", *p.DateFrom)
	}
	if p.DateTo != nil {
		q.Set("date_to", *p.DateTo)
	}
	if p.GroupBy != nil {
		q.Set("group_by", *p.GroupBy)
	}
	if p.BusinessHours != nil {
		q.Set("business_hours", strconv.FormatBool(*p.BusinessHours))
	}
	if p.Channels != nil {
		q.Set("channels", *p.Channels)
	}
	if p.Compare != nil {
		q.Set("compare", strconv.FormatBool(*p.Compare))
	}
	return q
}

func (s *ReportsService) report(ctx context.Context, path string, body map[string]any) (*Report, error) {
	var out Report
	if err := s.client.post(ctx, reportsBasePath+path, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Messages runs the messages activity report
// (POST /api/v1/reports/messages/).
func (s *ReportsService) Messages(ctx context.Context, params MessagesReportParams) (*Report, error) {
	return s.report(ctx, "/messages/", params.body())
}

// Channels runs the per-channel activity report
// (POST /api/v1/reports/channels/).
func (s *ReportsService) Channels(ctx context.Context, params ChannelsReportParams) (*Report, error) {
	return s.report(ctx, "/channels/", params.body())
}

// Clients runs the clients report (POST /api/v1/reports/clients/).
func (s *ReportsService) Clients(ctx context.Context, params ClientsReportParams) (*Report, error) {
	return s.report(ctx, "/clients/", params.body())
}

// Users runs the users report (POST /api/v1/reports/users/).
func (s *ReportsService) Users(ctx context.Context, params UsersReportParams) (*Report, error) {
	return s.report(ctx, "/users/", dateRangeReportBody(params.DateFrom, params.DateTo, params.Search))
}

// Bulks runs the bulk batches report (POST /api/v1/reports/bulks/).
func (s *ReportsService) Bulks(ctx context.Context, params BulksReportParams) (*Report, error) {
	return s.report(ctx, "/bulks/", dateRangeReportBody(params.DateFrom, params.DateTo, params.Search))
}

// SpecificBulks reports on a single bulk batch
// (POST /api/v1/reports/specific-bulks/).
func (s *ReportsService) SpecificBulks(ctx context.Context, params SpecificBulksReportParams) (*Report, error) {
	return s.report(ctx, "/specific-bulks/", params.body())
}

// Subscriptions runs the push subscriptions report
// (POST /api/v1/reports/subscriptions/).
func (s *ReportsService) Subscriptions(ctx context.Context, params SubscriptionsReportParams) (*Report, error) {
	return s.report(ctx, "/subscriptions/", params.body())
}

// AWSUsage fetches AWS usage statistics
// (POST /api/v1/reports/aws-usage-statistics/ — the page is a query
// parameter and the request has no body).
func (s *ReportsService) AWSUsage(ctx context.Context, params AWSUsageReportParams) (*Report, error) {
	q := url.Values{}
	if params.Page != nil {
		q.Set("page", strconv.Itoa(*params.Page))
	}
	var out Report
	if err := s.client.do(ctx, requestSpec{
		method: http.MethodPost,
		path:   reportsBasePath + "/aws-usage-statistics/",
		query:  q,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Conversations runs the Live Desk conversations report
// (GET /api/v1/reports/conversations/) — support-desk metrics with an
// agent / channel / team / label breakdown. GroupBy is one of "agent",
// "channel", "team" or "label". Parameters are sent as query string and
// the request has no body.
func (s *ReportsService) Conversations(ctx context.Context, params ConversationsReportParams) (*ConversationsReport, error) {
	var out ConversationsReport
	if err := s.client.get(ctx, reportsBasePath+"/conversations/", params.query(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Balance fetches the upstream balance for a provider account
// (GET /api/v1/reports/balance/{slug}/).
func (s *ReportsService) Balance(ctx context.Context, slug string) (*ProviderBalance, error) {
	var out ProviderBalance
	if err := s.client.get(ctx, reportsBasePath+"/balance/"+url.PathEscape(slug)+"/", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
