package parser

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ro-dosar/internal/domain"
)

// PDFParser parses PDF text content to extract document and appointment data
type PDFParser struct{}

// NewPDFParser creates a new PDF parser
func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

// DetectFileTypeFromContent detects the actual file type based on PDF content
// This is more reliable than URL-based detection since some files are mis-labeled on the website
func (p *PDFParser) DetectFileTypeFromContent(text string) string {
	// Check first 2000 characters for headers (some PDFs have headers spread across many lines)
	header := text
	if len(text) > 2000 {
		header = text[:2000]
	}
	headerLower := strings.ToLower(header)

	// Application files have table headers: "NR. DOSAR", "DATA ÎNREGISTRĂRII", "TERMEN", "SOLUTIE"
	hasNrDosar := strings.Contains(headerLower, "nr. dosar") || strings.Contains(headerLower, "nr.dosar")
	hasDataInreg := strings.Contains(headerLower, "data înregistrării") || strings.Contains(headerLower, "data inregistrarii")
	hasTermen := strings.Contains(headerLower, "termen")
	hasSolutie := strings.Contains(headerLower, "solutie") || strings.Contains(headerLower, "soluție")

	if hasNrDosar && (hasDataInreg || (hasTermen && hasSolutie)) {
		return "APPLICATION"
	}

	// Appointment result files have "Rezultate interviu" header
	if strings.Contains(headerLower, "rezultate interviu") || strings.Contains(headerLower, "rezultat interviu") {
		return "APPOINTMENT_RESULT"
	}

	// Appointment invitation files have "Lista" or "Invitati" or "Programare"
	if strings.Contains(headerLower, "lista") && (strings.Contains(headerLower, "interviu") || strings.Contains(headerLower, "invitat")) {
		return "APPOINTMENT_INVITATION"
	}

	// Default: unknown - caller should decide what to do
	return ""
}

// ApplicationRecord represents a parsed application record
type ApplicationRecord struct {
	DocumentNumber domain.DocumentNumber
	RegisteredAt   time.Time
	Category       domain.Category
	Term           *time.Time
	SolutionNumber *domain.DocumentNumber
}

// AppointmentRecord represents a parsed appointment record
type AppointmentRecord struct {
	DocumentNumber domain.DocumentNumber
	Date           time.Time
	Time           *time.Time
	Result         *string
	Type           domain.AppointmentType
}

var (
	// Pattern for document number: 10435/A/2025 or 10435\A\2025 or with spaces
	docNumPattern = regexp.MustCompile(`(\d+)\s*[/\\]\s*([A-Za-z]+)\s*[/\\]\s*(\d{4})`)

	// Pattern for document number with spaces in year: 82008/A/20 19 -> 82008/A/2019
	// Captures: number, category, first part of year, second part of year
	docNumWithSpacesPattern = regexp.MustCompile(`(\d+)\s*[/\\]\s*([A-Za-z]+)\s*[/\\]\s*(\d{2})\s+(\d{2})`)

	// Pattern for date: DD.MM.YYYY, DD/MM/YYYY, or DD-MM-YYYY (with optional spaces)
	datePattern = regexp.MustCompile(`(\d{1,2})\s*[./-]\s*(\d{1,2})\s*[./-]\s*(\d{4})`)

	// Pattern for time: HH:MM or HH.MM or just HH:MM without leading zero
	timePattern = regexp.MustCompile(`(\d{1,2})[:.](\d{2})`)

	// Results keywords - preserve original text as much as possible
	// Only normalize very common variations to improve consistency
	resultPatterns = map[string]string{
		"aviz pozitiv": "Aviz pozitiv",
		"aviz negativ": "Aviz negativ",
		"absent":       "Absent",
		"amanare":      "Amânare",
		"amânare":      "Amânare",
		"respins":      "Respins",
		"admis":        "Admis",
	}
)

// ParseApplicationsFromText parses application records from extracted PDF text
func (p *PDFParser) ParseApplicationsFromText(text string, category domain.Category) ([]ApplicationRecord, error) {
	var records []ApplicationRecord

	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()

		// Skip header lines and empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "NR. DOSAR") || strings.Contains(line, "DATA ÎNREGISTRĂRII") {
			continue
		}

		// Try to extract document number
		docNumMatch := docNumPattern.FindStringSubmatch(line)
		if docNumMatch == nil {
			continue
		}

		docNum, err := domain.ParseDocumentNumber(fmt.Sprintf("%s/%s/%s", docNumMatch[1], docNumMatch[2], docNumMatch[3]))
		if err != nil {
			continue
		}

		// Find all dates in the line
		allDates := datePattern.FindAllStringSubmatch(line, -1)
		if len(allDates) == 0 {
			continue
		}

		// First date is registration date
		day, _ := strconv.Atoi(allDates[0][1])
		month, _ := strconv.Atoi(allDates[0][2])
		year, _ := strconv.Atoi(allDates[0][3])
		registeredAt := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

		record := ApplicationRecord{
			DocumentNumber: docNum,
			RegisteredAt:   registeredAt,
			Category:       category,
		}

		// Second date is term date (if exists)
		if len(allDates) > 1 {
			tDay, _ := strconv.Atoi(allDates[1][1])
			tMonth, _ := strconv.Atoi(allDates[1][2])
			tYear, _ := strconv.Atoi(allDates[1][3])
			term := time.Date(tYear, time.Month(tMonth), tDay, 0, 0, 0, 0, time.UTC)
			record.Term = &term
		}

		// Check for solution number (second document number in line after the main one)
		allDocNums := docNumPattern.FindAllStringSubmatch(line, -1)
		if len(allDocNums) > 1 {
			solMatch := allDocNums[1]
			solNum, err := domain.ParseDocumentNumber(fmt.Sprintf("%s/%s/%s", solMatch[1], solMatch[2], solMatch[3]))
			if err == nil {
				record.SolutionNumber = &solNum
			}
		}

		records = append(records, record)
	}

	return records, scanner.Err()
}

// ParseAppointmentsFromText parses appointment records from extracted PDF text
func (p *PDFParser) ParseAppointmentsFromText(text string, appointmentType domain.AppointmentType, appointmentDate *time.Time) ([]AppointmentRecord, error) {
	var records []AppointmentRecord

	// Extract date from text (look in first 20 lines to be more thorough)
	var extractedDate time.Time
	dateFound := false
	scanner := bufio.NewScanner(strings.NewReader(text))
	lineCount := 0

	// Collect first 20 lines for date extraction
	var firstLines []string
	for scanner.Scan() && lineCount < 20 {
		line := scanner.Text()
		firstLines = append(firstLines, line)

		// Look for date in lines (especially those with "Rezultate", "interviu", "Lista", or dates)
		if !dateFound {
			// Check for common date indicator words in the line
			lowerLine := strings.ToLower(line)
			hasDateIndicators := strings.Contains(lowerLine, "rezultate") ||
				strings.Contains(lowerLine, "interviu") ||
				strings.Contains(lowerLine, "lista") ||
				strings.Contains(lowerLine, "data") ||
				strings.Contains(lowerLine, "programare")

			dateMatch := datePattern.FindStringSubmatch(line)
			if dateMatch != nil && (hasDateIndicators || lineCount < 5) {
				day, _ := strconv.Atoi(dateMatch[1])
				month, _ := strconv.Atoi(dateMatch[2])
				year, _ := strconv.Atoi(dateMatch[3])
				extractedDate = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
				dateFound = true
			}
		}
		lineCount++
	}

	// If still not found, try a broader search in the first 20 lines
	if !dateFound {
		fullText := strings.Join(firstLines, " ")
		dateMatch := datePattern.FindStringSubmatch(fullText)
		if dateMatch != nil {
			day, _ := strconv.Atoi(dateMatch[1])
			month, _ := strconv.Atoi(dateMatch[2])
			year, _ := strconv.Atoi(dateMatch[3])
			extractedDate = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			dateFound = true
		}
	}

	// Fall back to appointmentDate if provided and no date found in text
	if !dateFound && appointmentDate != nil {
		extractedDate = *appointmentDate
		dateFound = true
	}

	// If no date at all, return error
	if !dateFound {
		return nil, fmt.Errorf("could not extract appointment date from text or filename")
	}

	// Parse appointment records
	scanner = bufio.NewScanner(strings.NewReader(text))
	var currentLine string
	var nextLines []string
	lineNum := 0

	// Collect all lines
	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	// Process lines looking for document numbers
	for i := 0; i < len(allLines); i++ {
		currentLine = allLines[i]

		// Skip empty lines and header lines
		if strings.TrimSpace(currentLine) == "" {
			continue
		}
		if strings.Contains(currentLine, "Rezultate") || strings.Contains(currentLine, "NR. DOSAR") {
			continue
		}

		// Try to extract document number (try normal pattern first, then with-spaces pattern)
		docNumMatch := docNumPattern.FindStringSubmatch(currentLine)
		var docNumStr string
		if docNumMatch != nil {
			docNumStr = fmt.Sprintf("%s/%s/%s", docNumMatch[1], docNumMatch[2], docNumMatch[3])
		} else {
			// Try pattern with spaces in year (e.g., "82008/A/20 19")
			docNumSpacesMatch := docNumWithSpacesPattern.FindStringSubmatch(currentLine)
			if docNumSpacesMatch != nil {
				// Combine year parts: "20" + "19" = "2019"
				docNumStr = fmt.Sprintf("%s/%s/%s%s", docNumSpacesMatch[1], docNumSpacesMatch[2], docNumSpacesMatch[3], docNumSpacesMatch[4])
			} else {
				continue
			}
		}

		docNum, err := domain.ParseDocumentNumber(docNumStr)
		if err != nil {
			continue
		}

		record := AppointmentRecord{
			DocumentNumber: docNum,
			Date:           extractedDate,
			Type:           appointmentType,
		}

		// Collect next 3 lines for context (result or time might be on next line)
		nextLines = []string{}
		for j := 1; j <= 3 && i+j < len(allLines); j++ {
			nextLines = append(nextLines, allLines[i+j])
		}

		// For invitations, extract time from current line or next lines
		if appointmentType == domain.AppointmentTypeInvitation {
			fullContext := currentLine + " " + strings.Join(nextLines, " ")
			timeMatch := timePattern.FindStringSubmatch(fullContext)
			if timeMatch != nil {
				hour, _ := strconv.Atoi(timeMatch[1])
				minute, _ := strconv.Atoi(timeMatch[2])
				t := time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC)
				record.Time = &t
			}
		}

		// For results, extract result string (check current line first, then next lines)
		if appointmentType == domain.AppointmentTypeResult {
			// First, try to find result in current line only
			lowLine := strings.ToLower(currentLine)
			resultFound := false

			// Check for "T:" pattern first (preserve full term info)
			if strings.Contains(currentLine, "T:") {
				termMatch := regexp.MustCompile(`T:\s*([^\s]+(?:\s+[^\s]+)*)`).FindStringSubmatch(currentLine)
				if termMatch != nil {
					result := "Termen " + strings.TrimSpace(termMatch[1])
					record.Result = &result
					resultFound = true
				}
			}

			// Check for "Termen" pattern (more flexible, case-insensitive)
			// Handles formats: "Termen 17.10.2023", "- Termen 17.10.2023", "termen: 17.10.2023"
			if !resultFound && strings.Contains(strings.ToLower(currentLine), "termen") {
				termMatch := regexp.MustCompile(`(?i)[-–]?\s*termen\s*[:\-]?\s*(\d{1,2}[./]\d{1,2}[./]\d{4})`).FindStringSubmatch(currentLine)
				if termMatch != nil {
					result := "Termen " + strings.TrimSpace(termMatch[1])
					record.Result = &result
					resultFound = true
				}
			}

			// Then check for standardized result patterns
			if !resultFound {
				for pattern, normalizedResult := range resultPatterns {
					if strings.Contains(lowLine, pattern) {
						record.Result = &normalizedResult
						resultFound = true
						break
					}
				}
			}

			// If not found in current line, check next lines (for multi-line format)
			if !resultFound && len(nextLines) > 0 {
				for _, nextLine := range nextLines {
					lowNext := strings.ToLower(nextLine)

					// Check for "T:" pattern in next lines
					if strings.Contains(nextLine, "T:") {
						termMatch := regexp.MustCompile(`T:\s*([^\s]+(?:\s+[^\s]+)*)`).FindStringSubmatch(nextLine)
						if termMatch != nil {
							result := "Termen " + strings.TrimSpace(termMatch[1])
							record.Result = &result
							break
						}
					}

					// Check for "Termen" pattern in next lines (case-insensitive)
					if strings.Contains(lowNext, "termen") {
						termMatch := regexp.MustCompile(`(?i)[-–]?\s*termen\s*[:\-]?\s*(\d{1,2}[./]\d{1,2}[./]\d{4})`).FindStringSubmatch(nextLine)
						if termMatch != nil {
							result := "Termen " + strings.TrimSpace(termMatch[1])
							record.Result = &result
							break
						}
					}

					// Check standardized patterns in next lines
					for pattern, normalizedResult := range resultPatterns {
						if strings.Contains(lowNext, pattern) {
							record.Result = &normalizedResult
							break
						}
					}

					if record.Result != nil {
						break
					}
				}
			}
		}

		records = append(records, record)
		lineNum++
	}

	return records, scanner.Err()
}

// ParseDateFromFilename extracts date from PDF filename as a fallback
// Handles formats like "27.01.2026.pdf", "26.01.2026 pdf", or "Rezultate-16-12-2020.pdf"
// Note: Date from PDF text content is preferred and more reliable
func ParseDateFromFilename(filename string) *time.Time {
	// Try the standard pattern first
	dateMatch := datePattern.FindStringSubmatch(filename)
	if dateMatch == nil {
		return nil
	}

	day, _ := strconv.Atoi(dateMatch[1])
	month, _ := strconv.Atoi(dateMatch[2])
	year, _ := strconv.Atoi(dateMatch[3])

	// Validate the date components (basic validation)
	if day < 1 || day > 31 || month < 1 || month > 12 || year < 2000 || year > 2030 {
		return nil
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &date
}

// OathEntry is one dossier scheduled for the oath (no category letter in the lists)
type OathEntry struct {
	Number int
	Year   int
}

// OathList is one parsed oath-schedule PDF
type OathList struct {
	Date    time.Time
	Time    *string // "15:04" format, nil when the header carries no hour
	Entries []OathEntry
}

var (
	oathDatePattern  = regexp.MustCompile(`LA DATA DE\s+(\d{1,2})\.(\d{1,2})\.(\d{4})`)
	oathHourPattern  = regexp.MustCompile(`ORA\s+(\d{1,2})[.:](\d{2})`)
	oathEntryPattern = regexp.MustCompile(`(?m)^\s*\d+\.?\s+(\d+)/(\d{4})\s*$`)
)

// ParseOathList parses the text of an oath-schedule PDF: the header carries
// the ceremony date (mandatory) and hour (optional), the table rows carry
// dossier numbers without a category letter; duplicates are family members
// sharing one dossier and are deduplicated
func ParseOathList(text string) (*OathList, error) {
	d := oathDatePattern.FindStringSubmatch(text)
	if d == nil {
		return nil, fmt.Errorf("oath list has no ceremony date")
	}
	day, _ := strconv.Atoi(d[1])
	month, _ := strconv.Atoi(d[2])
	year, _ := strconv.Atoi(d[3])
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return nil, fmt.Errorf("oath list has an invalid ceremony date: %s", d[0])
	}

	list := &OathList{Date: time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)}

	if h := oathHourPattern.FindStringSubmatch(text); h != nil {
		hh, _ := strconv.Atoi(h[1])
		t := fmt.Sprintf("%02d:%s", hh, h[2])
		list.Time = &t
	}

	seen := map[OathEntry]bool{}
	for _, m := range oathEntryPattern.FindAllStringSubmatch(text, -1) {
		number, _ := strconv.Atoi(m[1])
		entryYear, _ := strconv.Atoi(m[2])
		e := OathEntry{Number: number, Year: entryYear}
		if seen[e] {
			continue
		}
		seen[e] = true
		list.Entries = append(list.Entries, e)
	}

	return list, nil
}
