package parser

import (
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// PDFLink represents a parsed PDF link from the HTML page
type PDFLink struct {
	URL      string
	Title    string
	Category string // ART_8, ART_8_1, ART_8_2, ART_10, ART_11
	Type     string // APPLICATION, APPOINTMENT_INVITATION, APPOINTMENT_RESULT
}

// HTMLParser parses HTML pages to extract PDF links
type HTMLParser struct {
	baseURL string
}

// NewHTMLParser creates a new HTML parser
func NewHTMLParser(baseURL string) *HTMLParser {
	return &HTMLParser{baseURL: baseURL}
}

// ParsePDFLinks extracts all PDF links from an HTML page
// Looks inside <div class="eael-tabs-content"> for the relevant content
func (p *HTMLParser) ParsePDFLinks(r io.Reader) ([]PDFLink, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	var links []PDFLink
	seen := make(map[string]bool) // Track seen URLs to avoid duplicates

	// Process individual tab content items - each tab represents a category section
	// Use data-title-link attribute to identify each section
	doc.Find(".eael-tab-content-item").Each(func(i int, tabContent *goquery.Selection) {
		// Get section identifier from data-title-link attribute
		titleLink, _ := tabContent.Attr("data-title-link")
		if titleLink == "" {
			// Also try ID attribute
			titleLink, _ = tabContent.Attr("id")
		}

		// Determine category and type from tab identifier
		category := p.extractCategoryFromContext(titleLink, "", "")
		fileType := p.extractFileTypeFromContext(titleLink, "", "")

		// Find all PDF links in this section
		tabContent.Find("a[href$='.pdf'], a[href*='.pdf']").Each(func(j int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists {
				return
			}

			// Make absolute URL if needed
			fullURL := p.resolveURL(href)
			if fullURL == "" {
				return
			}

			// Skip duplicates
			if seen[fullURL] {
				return
			}
			seen[fullURL] = true

			title := strings.TrimSpace(s.Text())

			// If category wasn't determined from tab, try from URL/title
			linkCategory := category
			if linkCategory == "" {
				linkCategory = p.extractCategoryFromContext("", fullURL, title)
			}

			// Skip if we couldn't determine category
			if linkCategory == "" {
				return
			}

			// Refine file type from URL/title if it's still APPLICATION
			linkType := fileType
			if linkType == "APPLICATION" {
				urlType := p.extractFileTypeFromContext("", fullURL, title)
				if urlType != "APPLICATION" {
					linkType = urlType
				}
			}

			links = append(links, PDFLink{
				URL:      fullURL,
				Title:    title,
				Category: linkCategory,
				Type:     linkType,
			})
		})
	})

	// Fallback: if no tabs found, search the whole document
	if len(links) == 0 {
		doc.Find("a[href$='.pdf'], a[href*='.pdf']").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists {
				return
			}

			fullURL := p.resolveURL(href)
			if fullURL == "" || seen[fullURL] {
				return
			}
			seen[fullURL] = true

			title := strings.TrimSpace(s.Text())
			category := p.extractCategory(fullURL, title)
			fileType := p.extractFileType(fullURL, title)

			if category != "" {
				links = append(links, PDFLink{
					URL:      fullURL,
					Title:    title,
					Category: category,
					Type:     fileType,
				})
			}
		})
	}

	return links, nil
}

// resolveURL converts a relative URL to an absolute URL
func (p *HTMLParser) resolveURL(href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	base, err := url.Parse(p.baseURL)
	if err != nil {
		return ""
	}

	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}

	return base.ResolveReference(ref).String()
}

// extractCategoryFromContext determines category from section context
func (p *HTMLParser) extractCategoryFromContext(sectionHTML, pdfURL, title string) string {
	low := strings.ToLower(sectionHTML + " " + pdfURL + " " + title)

	// Check for specific article patterns in order of specificity
	// HTML uses patterns like "articolul-82-tab" for Article 8.2
	patterns := []struct {
		keywords []string
		category string
	}{
		{[]string{"articolul-82", "articolul 8.2", "art. 8.2", "art.8.2", "art_8_2", "art-8-2", "art.-8.2"}, "ART_8_2"},
		{[]string{"articolul-81", "articolul 8.1", "art. 8.1", "art.8.1", "art_8_1", "art-8-1", "art.-8.1"}, "ART_8_1"},
		{[]string{"articolul-11", "articolul 11", "art. 11", "art.11", "art_11", "art-11"}, "ART_11"},
		{[]string{"articolul-10", "articolul 10", "art. 10", "art.10", "art_10", "art-10"}, "ART_10"},
		{[]string{"articolul-8-tab", "articolul-8\"", "articolul 8", "art. 8", "art.8", "art_8", "art-8-tab", "invitatii-interviu-art-8", "rezultate-interviu-art-8"}, "ART_8"},
	}

	for _, p := range patterns {
		for _, kw := range p.keywords {
			if strings.Contains(low, kw) {
				return p.category
			}
		}
	}

	return ""
}

// extractFileTypeFromContext determines file type from section context
func (p *HTMLParser) extractFileTypeFromContext(sectionHTML, pdfURL, title string) string {
	low := strings.ToLower(sectionHTML + " " + pdfURL + " " + title)

	// Check for result keywords (REZULTATE INTERVIU)
	// HTML uses patterns like "rezultate-interviu-art-8-tab"
	resultKeywords := []string{"rezultate-interviu", "rezultate interviu", "rezultat interviu", "rezultate", "aviz pozitiv", "aviz negativ", "absent"}
	for _, kw := range resultKeywords {
		if strings.Contains(low, kw) {
			return "APPOINTMENT_RESULT"
		}
	}

	// Check for invitation keywords (INVITATII INTERVIU)
	// HTML uses patterns like "invitatii-interviu-art-8-tab"
	invitationKeywords := []string{"invitatii-interviu", "invitatii interviu", "invitati interviu", "lista persoane invitate", "lista interviu", "programare"}
	for _, kw := range invitationKeywords {
		if strings.Contains(low, kw) {
			return "APPOINTMENT_INVITATION"
		}
	}

	// Default to application (document status lists)
	return "APPLICATION"
}

// extractCategory attempts to extract the category from URL or title (fallback)
func (p *HTMLParser) extractCategory(pdfURL, title string) string {
	return p.extractCategoryFromContext("", pdfURL, title)
}

// extractFileType attempts to determine the file type from URL or title (fallback)
func (p *HTMLParser) extractFileType(pdfURL, title string) string {
	return p.extractFileTypeFromContext("", pdfURL, title)
}

// OrdinLink is one ordin PDF reference parsed from an Ordine listing page
type OrdinLink struct {
	Number int
	Letter string
	Date   time.Time
	URL    string
}

var (
	ordinTextPattern = regexp.MustCompile(`^\s*(\d+)\s*[-.]?\s*([A-Za-z]{1,3})?\s*$`)
	ordinDatePattern = regexp.MustCompile(`(\d{1,2})\.(\d{1,2})\.(\d{4})`)
)

// ExtractOrdinLinks parses an Ordine listing page: the anchor text carries
// the ordin number ("2637P"), the PDF filename carries the date
// ("Ordin-2637P-12.08.2026-art-11.pdf"). Anchors without a parseable number
// or date are skipped — the index is best-effort. pdfAnchors counts every
// anchor whose href ends in ".pdf", regardless of whether it was extracted,
// so callers can measure how many candidates were skipped.
func ExtractOrdinLinks(htmlContent, baseURL string) (links []OrdinLink, pdfAnchors int, err error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, 0, err
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, 0, err
	}

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(href)), ".pdf") {
			return
		}
		pdfAnchors++

		// WordPress anchor text often contains non-breaking spaces (U+00A0)
		// instead of regular spaces; Go's \s does not match them.
		text := strings.ReplaceAll(s.Text(), " ", " ")

		m := ordinTextPattern.FindStringSubmatch(text)
		if m == nil {
			return
		}
		number, err := strconv.Atoi(m[1])
		if err != nil {
			return
		}
		letter := strings.ToUpper(m[2])
		if letter == "" {
			letter = "P"
		}

		d := ordinDatePattern.FindStringSubmatch(href)
		if d == nil {
			return
		}
		day, _ := strconv.Atoi(d[1])
		month, _ := strconv.Atoi(d[2])
		year, _ := strconv.Atoi(d[3])
		if month < 1 || month > 12 || day < 1 || day > 31 {
			return
		}

		ref, err := url.Parse(strings.TrimSpace(href))
		if err != nil {
			return
		}

		links = append(links, OrdinLink{
			Number: number,
			Letter: letter,
			Date:   time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC),
			URL:    base.ResolveReference(ref).String(),
		})
	})

	return links, pdfAnchors, nil
}
