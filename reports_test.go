package silon

import (
	"reflect"
	"testing"
)

var reportResponse = map[string]any{
	"report_data": []any{
		map[string]any{"client_id": "cust_001", "status": "sent", "retries": 0},
	},
	"total_items": 1,
	"total_pages": 1,
	"page":        1,
	"report_type": "phone",
}

func TestReportsMessages(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, reportResponse)))
	c := newTestClient(t, m)

	report, err := c.Reports.Messages(t.Context(), MessagesReportParams{
		ReportType: "phone",
		DateFrom:   String("2026-06-01"),
		Status:     []string{"sent", "read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalItems != 1 || report.TotalPages != 1 || report.Page != 1 {
		t.Errorf("report = %+v", report)
	}
	if report.ReportType != "phone" {
		t.Errorf("ReportType = %q", report.ReportType)
	}
	if len(report.ReportData) != 1 || report.ReportData[0]["client_id"] != "cust_001" {
		t.Errorf("ReportData = %v", report.ReportData)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/reports/messages/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"report_type": "phone",
		"date_from":   "2026-06-01",
		"status":      []any{"sent", "read"},
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestReportsChannels(t *testing.T) {
	response := map[string]any{
		"report_data": []any{}, "total_items": 0, "total_pages": 0, "page": 1,
		"report_type": nil,
	}
	m := newMockAPI(t, always(jsonStub(200, response)))
	c := newTestClient(t, m)

	report, err := c.Reports.Channels(t.Context(), ChannelsReportParams{
		ChannelName: []string{"whatsapp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ReportType != "" {
		t.Errorf("ReportType = %q, want empty for a null report_type", report.ReportType)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/reports/channels/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"channel_name": []any{"whatsapp"}}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestReportsClients(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, reportResponse)))
	c := newTestClient(t, m)

	_, err := c.Reports.Clients(t.Context(), ClientsReportParams{
		DateFrom:        String("2026-06-01"),
		Search:          String("sara"),
		Language:        []string{"en"},
		Device:          []string{"android"},
		WebSubscription: String("subscribed"),
	})
	if err != nil {
		t.Fatal(err)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/reports/clients/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{
		"date_from":        "2026-06-01",
		"search":           "sara",
		"language":         []any{"en"},
		"device":           []any{"android"},
		"web_subscription": "subscribed",
	}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestReportsUsers(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, reportResponse)))
	c := newTestClient(t, m)

	_, err := c.Reports.Users(t.Context(), UsersReportParams{Search: String("sara")})
	if err != nil {
		t.Fatal(err)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/reports/users/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"search": "sara"}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestReportsBulks(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, reportResponse)))
	c := newTestClient(t, m)

	_, err := c.Reports.Bulks(t.Context(), BulksReportParams{
		DateFrom: String("2026-06-01"),
		DateTo:   String("2026-06-30"),
	})
	if err != nil {
		t.Fatal(err)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/reports/bulks/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"date_from": "2026-06-01", "date_to": "2026-06-30"}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestReportsSpecificBulks(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, reportResponse)))
	c := newTestClient(t, m)

	_, err := c.Reports.SpecificBulks(t.Context(), SpecificBulksReportParams{
		BulksFile: 12,
		Status:    []string{"SENT"},
	})
	if err != nil {
		t.Fatal(err)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/reports/specific-bulks/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"bulks_file": float64(12), "status": []any{"SENT"}}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestReportsSubscriptions(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, reportResponse)))
	c := newTestClient(t, m)

	_, err := c.Reports.Subscriptions(t.Context(), SubscriptionsReportParams{
		ReportType: "mobile",
		DeviceType: []string{"android"},
	})
	if err != nil {
		t.Fatal(err)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/reports/subscriptions/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	want := map[string]any{"report_type": "mobile", "device_type": []any{"android"}}
	if got := last.jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestReportsAWSUsagePageQueryParamNoBody(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, reportResponse)))
	c := newTestClient(t, m)

	report, err := c.Reports.AWSUsage(t.Context(), AWSUsageReportParams{Page: Int(3)})
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalItems != 1 {
		t.Errorf("TotalItems = %d", report.TotalItems)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/reports/aws-usage-statistics/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if got := last.query.Get("page"); got != "3" {
		t.Errorf("page query param = %q, want %q", got, "3")
	}
	if len(last.body) != 0 {
		t.Errorf("request body = %q, want empty", last.body)
	}
}

func TestReportsBalance(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{"balance": "12.50"})))
	c := newTestClient(t, m)

	balance, err := c.Reports.Balance(t.Context(), "twilio-main")
	if err != nil {
		t.Fatal(err)
	}
	if balance.Balance != "12.50" {
		t.Errorf("Balance = %q", balance.Balance)
	}

	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/reports/balance/twilio-main/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if len(last.body) != 0 {
		t.Errorf("request body = %q, want empty", last.body)
	}
}

func TestReportsConversations(t *testing.T) {
	response := map[string]any{
		"group_by":       "agent",
		"business_hours": false,
		"totals":         map[string]any{"resolutions_count": 3, "csat_average": 4.5},
		"rows":           []any{map[string]any{"key": 1, "name": "Sara", "resolutions_count": 3}},
	}
	m := newMockAPI(t, always(jsonStub(200, response)))
	c := newTestClient(t, m)

	report, err := c.Reports.Conversations(t.Context(), ConversationsReportParams{
		GroupBy: String("agent"),
		Compare: Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := report.Totals["resolutions_count"]; !ok {
		t.Errorf("Totals = %v, missing resolutions_count", report.Totals)
	}
	if len(report.Rows) != 1 || report.Rows[0]["name"] != "Sara" {
		t.Errorf("Rows = %v", report.Rows)
	}

	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/reports/conversations/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	if got := last.query.Get("group_by"); got != "agent" {
		t.Errorf("group_by query param = %q, want %q", got, "agent")
	}
	if got := last.query.Get("compare"); got != "true" {
		t.Errorf("compare query param = %q, want %q", got, "true")
	}
	if len(last.body) != 0 {
		t.Errorf("request body = %q, want empty", last.body)
	}
}
