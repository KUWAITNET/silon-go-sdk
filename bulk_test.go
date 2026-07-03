package silon

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var bulkBatchResponse = map[string]any{
	"id":         12,
	"filename":   "batch.csv",
	"success":    8,
	"total":      10,
	"channels":   []any{"sms"},
	"created_at": "2026-07-01T09:00:00Z",
	"sent_at":    nil,
	"timezone":   "Asia/Kuwait",
}

var bulkSendResponse = map[string]any{
	"ok":       1,
	"message":  "Queued",
	"bulk_id":  12,
	"queued":   10,
	"failed":   0,
	"filename": "batch.csv",
}

var bulkUploadResponse = map[string]any{
	"name":              "0d9f.csv",
	"original_filename": "contacts.csv",
	"size":              33,
	"modified_at":       "2026-07-01T00:00:00Z",
}

func TestBulkList(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, []any{bulkBatchResponse})))
	c := newTestClient(t, m)

	batches, err := c.Bulk.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("len = %d, want 1", len(batches))
	}
	b := batches[0]
	if b.ID != 12 || b.Success != 8 || b.Total != 10 || b.Timezone != "Asia/Kuwait" {
		t.Errorf("batch = %+v", b)
	}
	wantCreated := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	if b.CreatedAt == nil || !b.CreatedAt.Equal(wantCreated) {
		t.Errorf("CreatedAt = %v", b.CreatedAt)
	}
	if b.SentAt != nil {
		t.Errorf("SentAt = %v, want nil", b.SentAt)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/bulk/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}

func TestBulkRetrieveDetail(t *testing.T) {
	detail := map[string]any{}
	for k, v := range bulkBatchResponse {
		detail[k] = v
	}
	detail["provider"] = "twilio"
	detail["applications"] = []any{}
	detail["web_applications"] = []any{}
	detail["sender"] = ""
	detail["template"] = []any{}
	detail["subject"] = ""
	detail["messages"] = []any{"Hello"}
	detail["scheduled_at"] = nil
	detail["recipients"] = []any{map[string]any{
		"id":           1,
		"client_id":    "cust_001",
		"phone_number": "+1",
		"email":        "",
		"status":       "SENT",
		"error":        "",
	}}
	m := newMockAPI(t, always(jsonStub(200, detail)))
	c := newTestClient(t, m)

	got, err := c.Bulk.Retrieve(t.Context(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 12 || got.Provider != "twilio" {
		t.Errorf("detail = %+v", got)
	}
	if len(got.Recipients) != 1 || got.Recipients[0].Status != "SENT" {
		t.Errorf("Recipients = %+v", got.Recipients)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/bulk/12/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}

func TestBulkSendInlineRecipients(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, bulkSendResponse)))
	c := newTestClient(t, m)

	result, err := c.Bulk.Send(t.Context(), BulkSendParams{
		Recipients: []map[string]any{
			{"client_id": "cust_001", "channel": "sms"},
			{"phone_number": "+1", "channel": "whatsapp"},
		},
		Channel:          String("sms,whatsapp"),
		Message:          String("Flash sale on now"),
		RemoveDuplicates: Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.BulkID != 12 || result.Queued != 10 || result.OK != 1 {
		t.Errorf("result = %+v", result)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/bulk/send/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	body := last.jsonBody(t)
	want := map[string]any{
		"recipients": []any{
			map[string]any{"client_id": "cust_001", "channel": "sms"},
			map[string]any{"phone_number": "+1", "channel": "whatsapp"},
		},
		"channel":           "sms,whatsapp",
		"message":           "Flash sale on now",
		"remove_duplicates": true,
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %v, want %v", body, want)
	}
	if _, present := body["bulk_file"]; present {
		t.Error("bulk_file must be dropped when omitted")
	}
	if len(last.query) != 0 {
		t.Errorf("query = %v, want empty", last.query)
	}
}

func TestBulkSendFromSavedFile(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, bulkSendResponse)))
	c := newTestClient(t, m)

	_, err := c.Bulk.Send(t.Context(), BulkSendParams{
		BulkFile: String("saved-uuid.csv"),
		Template: String("welcome"),
		Language: String("ar"),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"bulk_file": "saved-uuid.csv",
		"template":  "welcome",
		"language":  "ar",
	}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestBulkSendFullBodyFieldSet(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, bulkSendResponse)))
	c := newTestClient(t, m)

	_, err := c.Bulk.Send(t.Context(), BulkSendParams{
		BulkFile:         String("saved.csv"),
		Channel:          String("email"),
		Message:          String("hi"),
		Template:         String("welcome"),
		Subject:          String("Hello"),
		Sender:           String("noreply@acme.com"),
		Group:            String("vip"),
		Application:      String("acme-app"),
		WebApplication:   String("acme-widget"),
		Language:         String("en"),
		Files:            []string{"terms.pdf"},
		Name:             String("q3"),
		Expire:           Bool(true),
		RemoveDuplicates: Bool(false),
		ScheduledAt:      String("2026-07-04 09:00"),
		Timezone:         String("Asia/Kuwait"),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"bulk_file":         "saved.csv",
		"channel":           "email",
		"message":           "hi",
		"template":          "welcome",
		"subject":           "Hello",
		"sender":            "noreply@acme.com",
		"group":             "vip",
		"application":       "acme-app",
		"web_application":   "acme-widget",
		"language":          "en",
		"files":             []any{"terms.pdf"},
		"name":              "q3",
		"expire":            true,
		"remove_duplicates": false,
		"scheduled_at":      "2026-07-04 09:00",
		"timezone":          "Asia/Kuwait",
	}
	if got := m.lastCall(t).jsonBody(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestBulkSendRequiresExactlyOneSource(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, bulkSendResponse)))
	c := newTestClient(t, m)

	var baseErr *Error
	_, err := c.Bulk.Send(t.Context(), BulkSendParams{Channel: String("sms")})
	if !errors.As(err, &baseErr) || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("neither source: err = %v, want *Error mentioning 'exactly one'", err)
	}

	_, err = c.Bulk.Send(t.Context(), BulkSendParams{
		Recipients: []map[string]any{{"a": 1}},
		BulkFile:   String("x.csv"),
	})
	if !errors.As(err, &baseErr) || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("both sources: err = %v, want *Error mentioning 'exactly one'", err)
	}

	if m.callCount() != 0 {
		t.Errorf("validation must fail before any HTTP call; calls = %d", m.callCount())
	}
}

func TestBulkSendWhatsAppTemplateQueryParams(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, bulkSendResponse)))
	c := newTestClient(t, m)

	_, err := c.Bulk.Send(t.Context(), BulkSendParams{
		BulkFile:                  String("x.csv"),
		Provider:                  String("meta_cloud"),
		WhatsAppTemplate:          String("order_shipped"),
		WhatsAppTemplateLanguage:  String("en"),
		WhatsAppTemplateVariables: String(`{"1": "Sara"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	q := m.lastCall(t).query
	if q.Get("provider") != "meta_cloud" ||
		q.Get("whatsapp_template") != "order_shipped" ||
		q.Get("whatsapp_template_language") != "en" ||
		q.Get("whatsapp_template_variables") != `{"1": "Sara"}` {
		t.Errorf("query = %v", q)
	}
	// The query-only fields must not leak into the JSON body.
	body := m.lastCall(t).jsonBody(t)
	for _, key := range []string{"provider", "whatsapp_template", "whatsapp_template_language", "whatsapp_template_variables"} {
		if _, present := body[key]; present {
			t.Errorf("%s must be a query param, found in body", key)
		}
	}
}

func TestBulkFilesList(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"count": 1,
		"results": []any{map[string]any{
			"name": "a.csv", "size": 42, "modified_at": "2026-07-01T00:00:00Z",
		}},
	})))
	c := newTestClient(t, m)

	files, err := c.Bulk.Files.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if files.Count != 1 || len(files.Results) != 1 || files.Results[0].Name != "a.csv" {
		t.Errorf("files = %+v", files)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/bulk/files/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}

// parseMultipartFile decodes the recorded request's multipart body and
// returns the single form part.
func parseMultipartFile(t *testing.T, c call) (*multipart.Part, []byte) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(c.header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("Content-Type = %q (%v), want multipart/form-data", c.header.Get("Content-Type"), err)
	}
	reader := multipart.NewReader(bytes.NewReader(c.body), params["boundary"])
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("no multipart part: %v", err)
	}
	content, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("reading part: %v", err)
	}
	if _, err := reader.NextPart(); err != io.EOF {
		t.Fatalf("want exactly one part, got another (err=%v)", err)
	}
	return part, content
}

func TestBulkFilesUploadBytes(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, bulkUploadResponse)))
	c := newTestClient(t, m)

	uploaded, err := c.Bulk.Files.Upload(t.Context(), BulkFileUploadParams{
		Content: []byte("client_id,channel\ncust_001,sms\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.Name != "0d9f.csv" || uploaded.OriginalFilename != "contacts.csv" {
		t.Errorf("uploaded = %+v", uploaded)
	}

	last := m.lastCall(t)
	if last.method != "POST" || last.path != "/api/v1/bulk/files/" {
		t.Errorf("%s %s", last.method, last.path)
	}
	part, content := parseMultipartFile(t, last)
	if part.FormName() != "file" {
		t.Errorf("form name = %q, want \"file\"", part.FormName())
	}
	if part.FileName() != "recipients.csv" {
		t.Errorf("filename = %q, want default \"recipients.csv\"", part.FileName())
	}
	if got := part.Header.Get("Content-Type"); got != "text/csv" {
		t.Errorf("part Content-Type = %q, want \"text/csv\"", got)
	}
	if !bytes.Contains(content, []byte("cust_001,sms")) {
		t.Errorf("part content = %q", content)
	}
}

func TestBulkFilesUploadPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contacts.csv")
	if err := os.WriteFile(path, []byte("client_id\ncust_001\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newMockAPI(t, always(jsonStub(201, bulkUploadResponse)))
	c := newTestClient(t, m)

	if _, err := c.Bulk.Files.Upload(t.Context(), BulkFileUploadParams{Path: path}); err != nil {
		t.Fatal(err)
	}
	part, content := parseMultipartFile(t, m.lastCall(t))
	if part.FileName() != "contacts.csv" {
		t.Errorf("filename = %q, want the path's base name", part.FileName())
	}
	if !bytes.Contains(content, []byte("cust_001")) {
		t.Errorf("part content = %q", content)
	}
}

func TestBulkFilesUploadReaderWithCustomName(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, bulkUploadResponse)))
	c := newTestClient(t, m)

	_, err := c.Bulk.Files.Upload(t.Context(), BulkFileUploadParams{
		Reader:   strings.NewReader("client_id\n"),
		Filename: "q3-campaign.csv",
	})
	if err != nil {
		t.Fatal(err)
	}
	part, content := parseMultipartFile(t, m.lastCall(t))
	if part.FileName() != "q3-campaign.csv" {
		t.Errorf("filename = %q, want the explicit override", part.FileName())
	}
	if string(content) != "client_id\n" {
		t.Errorf("part content = %q", content)
	}
}

func TestBulkFilesUploadNamedReaderDerivesFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "from-disk.csv")
	if err := os.WriteFile(path, []byte("client_id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	m := newMockAPI(t, always(jsonStub(201, bulkUploadResponse)))
	c := newTestClient(t, m)

	if _, err := c.Bulk.Files.Upload(t.Context(), BulkFileUploadParams{Reader: f}); err != nil {
		t.Fatal(err)
	}
	part, _ := parseMultipartFile(t, m.lastCall(t))
	if part.FileName() != "from-disk.csv" {
		t.Errorf("filename = %q, want the *os.File's base name", part.FileName())
	}
}

func TestBulkFilesUploadRequiresExactlyOneSource(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(201, bulkUploadResponse)))
	c := newTestClient(t, m)

	var baseErr *Error
	_, err := c.Bulk.Files.Upload(t.Context(), BulkFileUploadParams{})
	if !errors.As(err, &baseErr) || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("no source: err = %v, want *Error mentioning 'exactly one'", err)
	}

	_, err = c.Bulk.Files.Upload(t.Context(), BulkFileUploadParams{
		Content: []byte("a"),
		Reader:  strings.NewReader("b"),
	})
	if !errors.As(err, &baseErr) || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("two sources: err = %v, want *Error mentioning 'exactly one'", err)
	}

	if m.callCount() != 0 {
		t.Errorf("validation must fail before any HTTP call; calls = %d", m.callCount())
	}
}

func TestBulkFilesUploadNotRetried(t *testing.T) {
	// A multipart POST carries no Idempotency-Key, so it must not be
	// replayed even on a retryable status.
	m := newMockAPI(t, always(jsonStub(http.StatusServiceUnavailable, map[string]any{})))
	c := newTestClient(t, m, WithMaxRetries(2))
	captureSleeps(c)

	_, err := c.Bulk.Files.Upload(t.Context(), BulkFileUploadParams{Content: []byte("x\n")})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want a 503 *APIError", err)
	}
	if m.callCount() != 1 {
		t.Errorf("calls = %d, want 1 (plain POST never retried)", m.callCount())
	}
}

func TestBulkRecipientRetrieve(t *testing.T) {
	m := newMockAPI(t, always(jsonStub(200, map[string]any{
		"id":           5,
		"file_name":    "batch.csv",
		"status":       "SENT",
		"channel":      "sms",
		"provider":     "twilio",
		"application":  "",
		"web_app":      "",
		"sender":       "",
		"template":     "",
		"subject":      "",
		"messages":     "Hello",
		"created_at":   "2026-07-01T09:00:00Z",
		"scheduled_at": "2026-07-01T09:00:00Z",
		"sent_at":      nil,
	})))
	c := newTestClient(t, m)

	recipient, err := c.Bulk.Recipients.Retrieve(t.Context(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if recipient.ID != 5 || recipient.Channel != "sms" || recipient.Messages != "Hello" {
		t.Errorf("recipient = %+v", recipient)
	}
	if recipient.SentAt != nil {
		t.Errorf("SentAt = %v, want nil", recipient.SentAt)
	}
	wantCreated := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	if recipient.CreatedAt == nil || !recipient.CreatedAt.Equal(wantCreated) {
		t.Errorf("CreatedAt = %v", recipient.CreatedAt)
	}
	last := m.lastCall(t)
	if last.method != "GET" || last.path != "/api/v1/bulk/recipient/5/" {
		t.Errorf("%s %s", last.method, last.path)
	}
}
