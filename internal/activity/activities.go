package activity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"ro-dosar/internal/domain"
	infrahttp "ro-dosar/internal/infrastructure/http"
	"ro-dosar/internal/infrastructure/pdf"
	"ro-dosar/internal/infrastructure/postgres"
	"ro-dosar/internal/repository"
	"ro-dosar/pkg/parser"
)

// Activities contains all Temporal activities
type Activities struct {
	httpClient      *infrahttp.Client
	browserClient   *infrahttp.BrowserClient
	htmlParser      *parser.HTMLParser
	pdfParser       *parser.PDFParser
	pdfExtractor    *pdf.TextExtractor
	documentRepo    repository.DocumentRepository
	appointmentRepo repository.AppointmentRepository
	parsedFileRepo  repository.ParsedFileRepository
	auditRepo       repository.DocumentAuditRepository
	pdfContentRepo  repository.PDFContentRepository
	ordinRepo       repository.OrdinRepository
}

// NewActivities creates a new Activities instance
func NewActivities(
	db *postgres.DB,
	httpClient *infrahttp.Client,
	browserClient *infrahttp.BrowserClient,
	baseURL string,
) *Activities {
	return &Activities{
		httpClient:      httpClient,
		browserClient:   browserClient,
		htmlParser:      parser.NewHTMLParser(baseURL),
		pdfParser:       parser.NewPDFParser(),
		pdfExtractor:    pdf.NewTextExtractor(),
		documentRepo:    postgres.NewDocumentRepository(db),
		appointmentRepo: postgres.NewAppointmentRepository(db),
		parsedFileRepo:  postgres.NewParsedFileRepository(db),
		auditRepo:       postgres.NewDocumentAuditRepository(db),
		pdfContentRepo:  postgres.NewPDFContentRepository(db),
		ordinRepo:       postgres.NewOrdinRepository(db),
	}
}

// FetchPageInput contains input for FetchPage activity
type FetchPageInput struct {
	URL string
}

// FetchPageOutput contains output from FetchPage activity
type FetchPageOutput struct {
	Content []byte
}

// FetchPage fetches an HTML page using headless browser
func (a *Activities) FetchPage(ctx context.Context, input FetchPageInput) (*FetchPageOutput, error) {
	content, _, err := a.browserClient.FetchPage(ctx, input.URL)
	if err != nil {
		return nil, err
	}
	return &FetchPageOutput{Content: content}, nil
}

// ExtractPDFLinksInput contains input for ExtractPDFLinks activity
type ExtractPDFLinksInput struct {
	Content []byte
}

// ExtractPDFLinksOutput contains output from ExtractPDFLinks activity
type ExtractPDFLinksOutput struct {
	Links []parser.PDFLink
}

// ExtractPDFLinks extracts PDF links from HTML content
func (a *Activities) ExtractPDFLinks(ctx context.Context, input ExtractPDFLinksInput) (*ExtractPDFLinksOutput, error) {
	reader := newBytesReader(input.Content)
	links, err := a.htmlParser.ParsePDFLinks(reader)
	if err != nil {
		return nil, err
	}
	return &ExtractPDFLinksOutput{Links: links}, nil
}

// CheckFileHashInput contains input for CheckFileHash activity
type CheckFileHashInput struct {
	URI string
}

// CheckFileHashOutput contains output from CheckFileHash activity
type CheckFileHashOutput struct {
	Exists      bool
	CurrentHash string
	Status      string // PARSED or NOT_FOUND
}

// CheckFileHash checks if a file exists and returns its hash and status
func (a *Activities) CheckFileHash(ctx context.Context, input CheckFileHashInput) (*CheckFileHashOutput, error) {
	file, err := a.parsedFileRepo.GetByURI(ctx, input.URI)
	if err != nil {
		if err == domain.ErrParsedFileNotFound {
			return &CheckFileHashOutput{Exists: false}, nil
		}
		return nil, err
	}
	return &CheckFileHashOutput{
		Exists:      true,
		CurrentHash: file.Hash,
		Status:      file.Status.String(),
	}, nil
}

// DownloadPDFInput contains input for DownloadPDF activity
type DownloadPDFInput struct {
	URL string
}

// DownloadPDFOutput contains output from DownloadPDF activity
// Note: Content is stored in database, only hash is returned to avoid size limits
type DownloadPDFOutput struct {
	Hash string
}

// DownloadPDF downloads a PDF file using the browser session and stores content in database
func (a *Activities) DownloadPDF(ctx context.Context, input DownloadPDFInput) (*DownloadPDFOutput, error) {
	content, _, err := a.browserClient.FetchPDF(ctx, input.URL)
	if err != nil {
		return nil, err
	}

	// Extract text to calculate hash from content instead of binary data
	// This ensures uniqueness based on actual content rather than PDF structure
	text, err := a.pdfExtractor.ExtractText(content)
	if err != nil {
		return nil, fmt.Errorf("failed to extract text for hash calculation: %w", err)
	}

	// Clean up text: remove extra whitespace, normalize line endings, etc.
	text = normalizeTextForHash(text)

	// Calculate hash from extracted text content
	hash := sha256.Sum256([]byte(text))
	hashStr := hex.EncodeToString(hash[:])

	// Store content in database to avoid passing large data between workflows
	if err := a.pdfContentRepo.Save(ctx, hashStr, content); err != nil {
		return nil, err
	}

	return &DownloadPDFOutput{Hash: hashStr}, nil
}

// ParseApplicationPDFInput contains input for ParseApplicationPDF activity
type ParseApplicationPDFInput struct {
	Hash     string // Content hash - content loaded from database
	Category string
}

// ParseApplicationPDFOutput contains output from ParseApplicationPDF activity
// Note: Records are saved directly to database to avoid size limits
type ParseApplicationPDFOutput struct {
	RecordCount int
}

// ParseApplicationPDF parses an application PDF and saves records to database
func (a *Activities) ParseApplicationPDF(ctx context.Context, input ParseApplicationPDFInput) (*ParseApplicationPDFOutput, error) {
	// Load content from database
	content, err := a.pdfContentRepo.Get(ctx, input.Hash)
	if err != nil {
		return nil, err
	}

	// Extract text from PDF
	text, err := a.pdfExtractor.ExtractText(content)
	if err != nil {
		return nil, err
	}

	// Parse the extracted text
	records, err := a.pdfParser.ParseApplicationsFromText(text, domain.Category(input.Category))
	if err != nil {
		return nil, err
	}

	// Save each record directly to database
	for _, record := range records {
		doc := domain.NewDocument(record.DocumentNumber, record.RegisteredAt, record.Category)
		if record.Term != nil {
			doc.SetTerm(*record.Term)
		}
		if record.SolutionNumber != nil {
			doc.SetSolutionNumber(*record.SolutionNumber)
		}

		// Check if document already exists for audit logging
		existingDoc, err := a.documentRepo.GetByNumber(ctx, record.DocumentNumber)
		isNew := err == domain.ErrDocumentNotFound

		// Save document (upsert)
		if err := a.documentRepo.Save(ctx, doc); err != nil {
			// Log error but continue processing
			continue
		}

		// Write audit log
		if isNew {
			_ = a.auditRepo.Log(ctx, record.DocumentNumber, repository.AuditActionCreate, nil, doc)
		} else if existingDoc != nil && doc.HasChanges(existingDoc) {
			_ = a.auditRepo.Log(ctx, record.DocumentNumber, repository.AuditActionUpdate, existingDoc, doc)
		}
	}

	return &ParseApplicationPDFOutput{RecordCount: len(records)}, nil
}

// ParseAppointmentPDFInput contains input for ParseAppointmentPDF activity
type ParseAppointmentPDFInput struct {
	Hash string // Content hash - content loaded from database
	URL  string // URL for extracting date from filename
	Type string // INVITATION or RESULT
}

// ParseAppointmentPDFOutput contains output from ParseAppointmentPDF activity
// Note: Records are saved directly to database to avoid size limits
type ParseAppointmentPDFOutput struct {
	RecordCount int
}

// ParseAppointmentPDF parses an appointment PDF and saves records to database
func (a *Activities) ParseAppointmentPDF(ctx context.Context, input ParseAppointmentPDFInput) (*ParseAppointmentPDFOutput, error) {
	// Load content from database
	content, err := a.pdfContentRepo.Get(ctx, input.Hash)
	if err != nil {
		return nil, err
	}

	// Extract text from PDF
	text, err := a.pdfExtractor.ExtractText(content)
	if err != nil {
		return nil, err
	}

	// Detect actual file type from content (website sometimes has mis-labeled files)
	actualType := a.pdfParser.DetectFileTypeFromContent(text)

	// If detected type doesn't match expected type and it's an APPLICATION file,
	// return 0 records. This should be handled by the workflow/routing logic.
	// The problem with large record counts happens when we try to parse APPLICATION
	// files as appointment files, which creates many incorrect appointment records.
	if actualType == "APPLICATION" && actualType != input.Type {
		return &ParseAppointmentPDFOutput{RecordCount: 0}, nil
	}

	// Extract date from filename (e.g., "Rezultate-interviu-18.11.2025.pdf" -> 2025-11-18)
	appointmentDate := parser.ParseDateFromFilename(input.URL)

	// Parse the extracted text with the expected type
	records, err := a.pdfParser.ParseAppointmentsFromText(text, domain.AppointmentType(input.Type), appointmentDate)
	if err != nil {
		return nil, err
	}

	// Save each record directly to database
	for _, record := range records {
		apt := domain.NewAppointment(record.DocumentNumber, record.Date, record.Type)
		if record.Time != nil {
			apt.SetTime(*record.Time)
		}
		if record.Result != nil {
			apt.SetResult(*record.Result)
		}

		// Save appointment (upsert)
		if err := a.appointmentRepo.Save(ctx, apt); err != nil {
			// Log error but continue processing
			continue
		}
	}

	return &ParseAppointmentPDFOutput{RecordCount: len(records)}, nil
}

// SaveParsedFileInput contains input for SaveParsedFile activity
type SaveParsedFileInput struct {
	URI      string
	Hash     string
	Category string
	Type     string
}

// SaveParsedFile saves a parsed file record
func (a *Activities) SaveParsedFile(ctx context.Context, input SaveParsedFileInput) error {
	file := domain.NewParsedFile(
		input.URI,
		input.Hash,
		domain.Category(input.Category),
		domain.FileType(input.Type),
	)
	return a.parsedFileRepo.Save(ctx, file)
}

// SaveNotFoundFileInput contains input for SaveNotFoundFile activity
type SaveNotFoundFileInput struct {
	URI      string
	Category string
	Type     string
}

// SaveNotFoundFile saves a record for a file that was not found (HTTP 404)
func (a *Activities) SaveNotFoundFile(ctx context.Context, input SaveNotFoundFileInput) error {
	file := domain.NewNotFoundFile(
		input.URI,
		domain.Category(input.Category),
		domain.FileType(input.Type),
	)
	return a.parsedFileRepo.Save(ctx, file)
}

// DeletePDFContentInput contains input for DeletePDFContent activity
type DeletePDFContentInput struct {
	Hash string
}

// DeletePDFContent removes PDF content from storage after processing
func (a *Activities) DeletePDFContent(ctx context.Context, input DeletePDFContentInput) error {
	return a.pdfContentRepo.Delete(ctx, input.Hash)
}

// GetDocumentInput contains input for GetDocument activity
type GetDocumentInput struct {
	DocumentNumber string
}

// GetDocumentOutput contains output from GetDocument activity
type GetDocumentOutput struct {
	Document *domain.Document
	Found    bool
}

// GetDocument retrieves a document by number
func (a *Activities) GetDocument(ctx context.Context, input GetDocumentInput) (*GetDocumentOutput, error) {
	docNum, err := domain.ParseDocumentNumber(input.DocumentNumber)
	if err != nil {
		return nil, err
	}

	doc, err := a.documentRepo.GetByNumber(ctx, docNum)
	if err != nil {
		if err == domain.ErrDocumentNotFound {
			return &GetDocumentOutput{Found: false}, nil
		}
		return nil, err
	}
	return &GetDocumentOutput{Document: doc, Found: true}, nil
}

// SaveDocumentInput contains input for SaveDocument activity
type SaveDocumentInput struct {
	DocumentNumber string
	RegisteredAt   string
	Category       string
	Term           *string
	SolutionNumber *string
}

// SaveDocument saves a document
func (a *Activities) SaveDocument(ctx context.Context, input SaveDocumentInput) error {
	docNum, err := domain.ParseDocumentNumber(input.DocumentNumber)
	if err != nil {
		return err
	}

	registeredAt, err := parseDate(input.RegisteredAt)
	if err != nil {
		return err
	}

	doc := domain.NewDocument(docNum, registeredAt, domain.Category(input.Category))

	if input.Term != nil {
		term, err := parseDate(*input.Term)
		if err != nil {
			return err
		}
		doc.SetTerm(term)
	}

	if input.SolutionNumber != nil {
		solNum, err := domain.ParseDocumentNumber(*input.SolutionNumber)
		if err != nil {
			return err
		}
		doc.SetSolutionNumber(solNum)
	}

	return a.documentRepo.Save(ctx, doc)
}

// SaveAuditLogInput contains input for SaveAuditLog activity
type SaveAuditLogInput struct {
	DocumentNumber string
	Action         string
	OldDocument    *domain.Document
	NewDocument    *domain.Document
}

// SaveAuditLog saves an audit log entry
func (a *Activities) SaveAuditLog(ctx context.Context, input SaveAuditLogInput) error {
	docNum, err := domain.ParseDocumentNumber(input.DocumentNumber)
	if err != nil {
		return err
	}

	return a.auditRepo.Log(ctx, docNum, repository.AuditAction(input.Action), input.OldDocument, input.NewDocument)
}

// GetAppointmentInput contains input for GetAppointment activity
type GetAppointmentInput struct {
	DocumentNumber string
	Date           string
	Type           string
}

// GetAppointmentOutput contains output from GetAppointment activity
type GetAppointmentOutput struct {
	Appointments []*domain.Appointment
}

// GetAppointment retrieves appointments by document and type
func (a *Activities) GetAppointment(ctx context.Context, input GetAppointmentInput) (*GetAppointmentOutput, error) {
	docNum, err := domain.ParseDocumentNumber(input.DocumentNumber)
	if err != nil {
		return nil, err
	}

	appointments, err := a.appointmentRepo.GetByDocumentAndType(ctx, docNum, domain.AppointmentType(input.Type))
	if err != nil {
		return nil, err
	}
	return &GetAppointmentOutput{Appointments: appointments}, nil
}

// SaveAppointmentInput contains input for SaveAppointment activity
type SaveAppointmentInput struct {
	DocumentNumber string
	Date           string
	Time           *string
	Result         *string
	Type           string
}

// SaveAppointment saves an appointment
func (a *Activities) SaveAppointment(ctx context.Context, input SaveAppointmentInput) error {
	docNum, err := domain.ParseDocumentNumber(input.DocumentNumber)
	if err != nil {
		return err
	}

	date, err := parseDate(input.Date)
	if err != nil {
		return err
	}

	appointment := domain.NewAppointment(docNum, date, domain.AppointmentType(input.Type))

	if input.Time != nil {
		t, err := parseTime(*input.Time)
		if err != nil {
			return err
		}
		appointment.SetTime(t)
	}

	if input.Result != nil {
		appointment.SetResult(*input.Result)
	}

	return a.appointmentRepo.Save(ctx, appointment)
}

// normalizeTextForHash normalizes text for hash calculation
// Removes extra whitespace and normalizes line endings to ensure consistent hashing
func normalizeTextForHash(text string) string {
	// Replace all types of whitespace with single space
	reg := regexp.MustCompile(`\s+`)
	normalized := reg.ReplaceAllString(text, " ")

	// Trim leading/trailing whitespace
	normalized = strings.TrimSpace(normalized)

	return normalized
}

// ExtractOrdineInput contains input for ExtractOrdine activity
type ExtractOrdineInput struct {
	Content []byte
	PageURL string
}

// ExtractOrdineOutput contains output from ExtractOrdine activity
type ExtractOrdineOutput struct {
	Ordins []repository.Ordin
}

// ExtractOrdine parses an Ordine listing page into indexable ordin records
func (a *Activities) ExtractOrdine(ctx context.Context, input ExtractOrdineInput) (*ExtractOrdineOutput, error) {
	links, err := parser.ExtractOrdinLinks(string(input.Content), input.PageURL)
	if err != nil {
		return nil, err
	}

	out := &ExtractOrdineOutput{}
	for _, l := range links {
		out.Ordins = append(out.Ordins, repository.Ordin{
			URL:        l.URL,
			Number:     l.Number,
			Letter:     l.Letter,
			Date:       l.Date,
			SourcePage: input.PageURL,
		})
	}
	return out, nil
}

// SaveOrdineInput contains input for SaveOrdine activity
type SaveOrdineInput struct {
	Ordins []repository.Ordin
}

// SaveOrdine upserts the extracted ordins
func (a *Activities) SaveOrdine(ctx context.Context, input SaveOrdineInput) error {
	return a.ordinRepo.SaveBatch(ctx, input.Ordins)
}
