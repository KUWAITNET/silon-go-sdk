package silon

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	bulkPath           = "/api/v1/bulk/"
	bulkFilesPath      = "/api/v1/bulk/files/"
	bulkSendPath       = "/api/v1/bulk/send/"
	bulkRecipientsPath = "/api/v1/bulk/recipient/"
)

// defaultBulkFilename is the multipart filename used when an upload's
// name cannot be derived from its source.
const defaultBulkFilename = "recipients.csv"

// BulkBatch is one row from GET /api/v1/bulk/.
type BulkBatch struct {
	// ID is the bulk batch id.
	ID int `json:"id"`

	// Filename is the stored CSV filename the batch was built from.
	Filename string `json:"filename"`

	// Success is how many recipients in this batch were sent/read.
	Success int `json:"success"`

	// Total is the total number of recipients in this batch.
	Total int `json:"total"`

	// Channels are the distinct channels used across the batch's
	// recipients.
	Channels []any `json:"channels,omitempty"`

	// CreatedAt is when the batch was created.
	CreatedAt *time.Time `json:"created_at,omitempty"`

	// SentAt is when the batch finished sending; nil until then.
	SentAt *time.Time `json:"sent_at,omitempty"`

	// Timezone is the IANA zone scheduled sends are interpreted in.
	Timezone string `json:"timezone,omitempty"`
}

// BulkRecipient is a per-recipient row embedded in a bulk batch detail.
type BulkRecipient struct {
	ID          int    `json:"id"`
	ClientID    string `json:"client_id,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	Email       string `json:"email,omitempty"`
	Status      string `json:"status,omitempty"`
	Error       string `json:"error,omitempty"`
}

// BulkBatchDetail is the full batch detail from GET /api/v1/bulk/{id}/.
type BulkBatchDetail struct {
	BulkBatch

	// Provider is the provider that handled the batch, if uniform.
	Provider string `json:"provider,omitempty"`

	// Applications are push app names used by the batch (push channel).
	Applications []any `json:"applications,omitempty"`

	// WebApplications are Web Push widget names used (web_push channel).
	WebApplications []any `json:"web_applications,omitempty"`

	// Sender is the from-identity the batch was sent from, if set.
	Sender string `json:"sender,omitempty"`

	// Template lists the templates used across the batch's recipients.
	Template []any `json:"template,omitempty"`

	// Subject is the subject line (email channel).
	Subject string `json:"subject,omitempty"`

	// Messages are the distinct rendered message bodies in the batch.
	Messages []any `json:"messages,omitempty"`

	// ScheduledAt is when the batch is/was scheduled to send.
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`

	// Recipients holds the per-recipient rows.
	Recipients []BulkRecipient `json:"recipients,omitempty"`
}

// BulkRecipientDetail is the body of GET /api/v1/bulk/recipient/{id}/.
type BulkRecipientDetail struct {
	// ID is the recipient row id.
	ID int `json:"id"`

	// FileName is the CSV filename this recipient came from.
	FileName string `json:"file_name,omitempty"`

	// Status is the delivery status of this recipient row.
	Status string `json:"status,omitempty"`

	// Channel this row was sent on, e.g. "sms" / "whatsapp".
	Channel string `json:"channel,omitempty"`

	// Provider that handled the send, if any.
	Provider string `json:"provider,omitempty"`

	// Application is the push app name (push channel), else empty.
	Application string `json:"application,omitempty"`

	// WebApp is the Web Push widget name (web_push channel), else empty.
	WebApp string `json:"web_app,omitempty"`

	// Sender is the from-identity this row was sent from, if set.
	Sender string `json:"sender,omitempty"`

	// Template used to render the message, if any.
	Template string `json:"template,omitempty"`

	// Subject line (email channel).
	Subject string `json:"subject,omitempty"`

	// Messages is the rendered message body sent to this recipient.
	Messages string `json:"messages,omitempty"`

	CreatedAt   *time.Time `json:"created_at,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
}

// BulkFile is one saved CSV listed by GET /api/v1/bulk/files/.
type BulkFile struct {
	// Name is the saved filename — pass it as BulkFile to
	// BulkService.Send.
	Name string `json:"name"`

	// Size is the file size in bytes.
	Size int `json:"size"`

	// ModifiedAt is when the file was last written.
	ModifiedAt time.Time `json:"modified_at"`
}

// BulkFileList is the body of GET /api/v1/bulk/files/.
type BulkFileList struct {
	// Count is the number of saved CSVs.
	Count int `json:"count"`

	// Results are the saved CSVs available for bulk sends.
	Results []BulkFile `json:"results"`
}

// BulkFileUpload is the 201 body of POST /api/v1/bulk/files/.
type BulkFileUpload struct {
	// Name is the UUID-based saved filename; pass it as BulkFile to
	// BulkService.Send.
	Name string `json:"name"`

	// OriginalFilename is the filename you uploaded, kept for your own
	// records.
	OriginalFilename string `json:"original_filename"`

	// Size is the file size in bytes.
	Size int `json:"size"`

	// ModifiedAt is when the upload was saved.
	ModifiedAt time.Time `json:"modified_at"`
}

// BulkSendResult is the success body of POST /api/v1/bulk/send/.
type BulkSendResult struct {
	// OK is 1 on success, 0 on failure.
	OK int `json:"ok"`

	// Message is a human-readable confirmation.
	Message string `json:"message"`

	// BulkID is the id of the created bulk batch.
	BulkID int `json:"bulk_id"`

	// Queued is how many recipients were queued for delivery.
	Queued int `json:"queued"`

	// Failed is how many recipients failed validation.
	Failed int `json:"failed"`

	// Filename is the stored CSV filename the batch was built from.
	Filename string `json:"filename"`
}

// BulkSendParams are the parameters for BulkService.Send.
//
// Exactly one of Recipients (inline rows) or BulkFile (a saved CSV name
// from BulkFilesService.Upload) is required. Nil fields are omitted from
// the request. Provider and the WhatsAppTemplate* fields are sent as
// query parameters, everything else in the JSON body.
type BulkSendParams struct {
	// Recipients are inline recipient rows; each is a column->value map.
	Recipients []map[string]any

	// BulkFile is the filename of a CSV previously saved via
	// BulkFilesService.Upload.
	BulkFile *string

	// Channel is the default channel(s), comma-separated
	// (e.g. "sms,whatsapp").
	Channel *string

	// Message is the inline message body, when not using a template.
	Message *string

	// Template is a message template slug.
	Template *string

	// Subject is the subject line (email channel).
	Subject *string

	// Sender overrides the from-identity (channel/provider dependent).
	Sender *string

	// Group targets a client group.
	Group *string

	// Application is the app slug for the push channel.
	Application *string

	// WebApplication is the widget slug for the web_push channel.
	WebApplication *string

	// Language is the two-letter code used to render the template.
	Language *string

	// Files are attachment names to include with each message.
	Files []string

	// Name is the base name for the generated CSV when sending inline
	// recipients.
	Name *string

	// Expire expires undelivered messages.
	Expire *bool

	// RemoveDuplicates drops duplicate recipients before sending.
	RemoveDuplicates *bool

	// ScheduledAt schedules the batch ("YYYY-MM-DD HH:MM"), interpreted
	// in Timezone.
	ScheduledAt *string

	// Timezone is the IANA zone ScheduledAt is interpreted in.
	Timezone *string

	// Provider is sent as a query parameter.
	Provider *string

	// WhatsAppTemplate is the WhatsApp template name (query parameter).
	WhatsAppTemplate *string

	// WhatsAppTemplateLanguage is the template language (query parameter).
	WhatsAppTemplateLanguage *string

	// WhatsAppTemplateVariables are the template variables, JSON-encoded
	// (query parameter).
	WhatsAppTemplateVariables *string
}

func (p BulkSendParams) body() (map[string]any, error) {
	if (p.Recipients == nil) == (p.BulkFile == nil) {
		return nil, &Error{
			Message: "Provide exactly one of 'recipients' (inline rows) or 'bulk_file' (a saved CSV name).",
		}
	}
	body := map[string]any{}
	if p.Recipients != nil {
		body["recipients"] = p.Recipients
	}
	if p.BulkFile != nil {
		body["bulk_file"] = *p.BulkFile
	}
	if p.Channel != nil {
		body["channel"] = *p.Channel
	}
	if p.Message != nil {
		body["message"] = *p.Message
	}
	if p.Template != nil {
		body["template"] = *p.Template
	}
	if p.Subject != nil {
		body["subject"] = *p.Subject
	}
	if p.Sender != nil {
		body["sender"] = *p.Sender
	}
	if p.Group != nil {
		body["group"] = *p.Group
	}
	if p.Application != nil {
		body["application"] = *p.Application
	}
	if p.WebApplication != nil {
		body["web_application"] = *p.WebApplication
	}
	if p.Language != nil {
		body["language"] = *p.Language
	}
	if p.Files != nil {
		body["files"] = p.Files
	}
	if p.Name != nil {
		body["name"] = *p.Name
	}
	if p.Expire != nil {
		body["expire"] = *p.Expire
	}
	if p.RemoveDuplicates != nil {
		body["remove_duplicates"] = *p.RemoveDuplicates
	}
	if p.ScheduledAt != nil {
		body["scheduled_at"] = *p.ScheduledAt
	}
	if p.Timezone != nil {
		body["timezone"] = *p.Timezone
	}
	return body, nil
}

func (p BulkSendParams) values() url.Values {
	q := url.Values{}
	if p.Provider != nil {
		q.Set("provider", *p.Provider)
	}
	if p.WhatsAppTemplate != nil {
		q.Set("whatsapp_template", *p.WhatsAppTemplate)
	}
	if p.WhatsAppTemplateLanguage != nil {
		q.Set("whatsapp_template_language", *p.WhatsAppTemplateLanguage)
	}
	if p.WhatsAppTemplateVariables != nil {
		q.Set("whatsapp_template_variables", *p.WhatsAppTemplateVariables)
	}
	return q
}

// BulkFileUploadParams are the parameters for BulkFilesService.Upload.
//
// Exactly one content source is required: Path (read from disk), Content
// (raw CSV bytes), or Reader (streamed). The multipart filename defaults
// to Filename, else the source's own name (Path's base name, or Name()
// of a Reader such as *os.File), else "recipients.csv".
type BulkFileUploadParams struct {
	// Path reads the CSV from a file on disk.
	Path string

	// Content is the raw CSV bytes.
	Content []byte

	// Reader streams the CSV; it is read fully at call time.
	Reader io.Reader

	// Filename overrides the multipart filename.
	Filename string
}

// filename resolves the multipart filename per the precedence documented
// on BulkFileUploadParams.
func (p BulkFileUploadParams) filename() string {
	if p.Filename != "" {
		return p.Filename
	}
	if p.Path != "" {
		return filepath.Base(p.Path)
	}
	if named, ok := p.Reader.(interface{ Name() string }); ok {
		if name := named.Name(); name != "" {
			return filepath.Base(name)
		}
	}
	return defaultBulkFilename
}

// content resolves the CSV bytes from whichever source is set, erroring
// unless exactly one of Path / Content / Reader was provided.
func (p BulkFileUploadParams) content() ([]byte, error) {
	sources := 0
	if p.Path != "" {
		sources++
	}
	if p.Content != nil {
		sources++
	}
	if p.Reader != nil {
		sources++
	}
	if sources != 1 {
		return nil, &Error{
			Message: "Provide exactly one of Path, Content, or Reader as the CSV source.",
		}
	}
	switch {
	case p.Path != "":
		data, err := os.ReadFile(p.Path)
		if err != nil {
			return nil, &Error{Message: "Could not read upload file: " + err.Error()}
		}
		return data, nil
	case p.Content != nil:
		return p.Content, nil
	default:
		data, err := io.ReadAll(p.Reader)
		if err != nil {
			return nil, &Error{Message: "Could not read upload content: " + err.Error()}
		}
		return data, nil
	}
}

// BulkFilesService manages the saved CSVs bulk sends are built from
// (/api/v1/bulk/files/). Access it via Client.Bulk.Files.
type BulkFilesService struct {
	client *Client
}

// List fetches the saved CSVs available for bulk sends
// (GET /api/v1/bulk/files/).
func (s *BulkFilesService) List(ctx context.Context) (*BulkFileList, error) {
	var out BulkFileList
	if err := s.client.get(ctx, bulkFilesPath, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Upload saves a CSV for later bulk sends (POST /api/v1/bulk/files/,
// multipart form field "file", part content type text/csv). Pass the
// returned Name as BulkSendParams.BulkFile.
func (s *BulkFilesService) Upload(ctx context.Context, params BulkFileUploadParams) (*BulkFileUpload, error) {
	content, err := params.content()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition",
		`form-data; name="file"; filename=`+strconv.Quote(params.filename()))
	partHeader.Set("Content-Type", "text/csv")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, &Error{Message: "Could not build multipart body: " + err.Error()}
	}
	if _, err := part.Write(content); err != nil {
		return nil, &Error{Message: "Could not build multipart body: " + err.Error()}
	}
	if err := writer.Close(); err != nil {
		return nil, &Error{Message: "Could not build multipart body: " + err.Error()}
	}

	var out BulkFileUpload
	if err := s.client.do(ctx, requestSpec{
		method:  http.MethodPost,
		path:    bulkFilesPath,
		rawBody: buf.Bytes(),
		rawType: writer.FormDataContentType(),
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BulkRecipientsService looks up individual bulk recipient rows
// (/api/v1/bulk/recipient/). Access it via Client.Bulk.Recipients.
type BulkRecipientsService struct {
	client *Client
}

// Retrieve fetches one bulk recipient row by id
// (GET /api/v1/bulk/recipient/{id}/).
func (s *BulkRecipientsService) Retrieve(ctx context.Context, recipientID int) (*BulkRecipientDetail, error) {
	var out BulkRecipientDetail
	if err := s.client.get(ctx, bulkRecipientsPath+strconv.Itoa(recipientID)+"/", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BulkService runs bulk (CSV) sends: batches, saved files, and
// per-recipient rows. Access it via Client.Bulk.
type BulkService struct {
	client *Client

	// Files manages the saved CSVs bulk sends are built from.
	Files *BulkFilesService

	// Recipients looks up individual bulk recipient rows.
	Recipients *BulkRecipientsService
}

// List fetches past bulk batches (GET /api/v1/bulk/ — the API returns a
// bare JSON array, not a paginated envelope).
func (s *BulkService) List(ctx context.Context) ([]BulkBatch, error) {
	var out []BulkBatch
	if err := s.client.get(ctx, bulkPath, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Retrieve fetches one bulk batch with its per-recipient rows
// (GET /api/v1/bulk/{id}/).
func (s *BulkService) Retrieve(ctx context.Context, bulkID int) (*BulkBatchDetail, error) {
	var out BulkBatchDetail
	if err := s.client.get(ctx, bulkPath+strconv.Itoa(bulkID)+"/", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Send starts a bulk batch (POST /api/v1/bulk/send/).
//
// Supply recipients either inline via params.Recipients (each row a
// column->value map) or by referencing a previously uploaded CSV via
// params.BulkFile — exactly one of the two, or a client-side *Error is
// returned.
//
// Deprecated: use MessagesService.SendBatch — inline rows via Messages,
// saved CSVs via File (upload with BulkFilesService.Upload, which stays
// current). This endpoint's behavior is frozen; there is no removal date.
func (s *BulkService) Send(ctx context.Context, params BulkSendParams) (*BulkSendResult, error) {
	body, err := params.body()
	if err != nil {
		return nil, err
	}
	var out BulkSendResult
	if err := s.client.do(ctx, requestSpec{
		method: http.MethodPost,
		path:   bulkSendPath,
		query:  params.values(),
		body:   body,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
