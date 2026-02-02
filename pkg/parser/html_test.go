package parser

import (
	"strings"
	"testing"
)

func TestParsePDFLinks(t *testing.T) {
	parser := NewHTMLParser("https://cetatenie.just.ro")

	html := `
<!DOCTYPE html>
<html>
<body>
<div class="eael-tabs-content">
	<div id="articolul-11-tab" class="eael-tab-content-item" data-title-link="articolul-11-tab">
		<a href="/wp-content/uploads/Art-11-2025.pdf">2025</a>
	</div>
	<div id="articolul-8-tab" class="eael-tab-content-item" data-title-link="articolul-8-tab">
		<a href="/wp-content/uploads/Acordare-art-8-2025.pdf">2025</a>
	</div>
	<div id="articolul-81-tab" class="eael-tab-content-item" data-title-link="articolul-81-tab">
		<a href="/wp-content/uploads/Art-8.1-2025.pdf">2025</a>
	</div>
	<div id="rezultate-interviu-art-8-tab" class="eael-tab-content-item" data-title-link="rezultate-interviu-art-8-tab">
		<a href="/wp-content/uploads/Rezultate-interviu-27.01.2026.pdf">27.01.2026</a>
	</div>
	<div id="invitatii-interviu-art-8-tab" class="eael-tab-content-item" data-title-link="invitatii-interviu-art-8-tab">
		<a href="/wp-content/uploads/Lista-interviu-04.02.2026.pdf">04.02.2026</a>
	</div>
</div>
</body>
</html>
`

	links, err := parser.ParsePDFLinks(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParsePDFLinks() error = %v", err)
	}

	if len(links) != 5 {
		t.Fatalf("ParsePDFLinks() got %d links, want 5", len(links))
	}

	// Check categories and types
	tests := []struct {
		url      string
		category string
		fileType string
	}{
		{"/wp-content/uploads/Art-11-2025.pdf", "ART_11", "APPLICATION"},
		{"/wp-content/uploads/Acordare-art-8-2025.pdf", "ART_8", "APPLICATION"},
		{"/wp-content/uploads/Art-8.1-2025.pdf", "ART_8_1", "APPLICATION"},
		{"/wp-content/uploads/Rezultate-interviu-27.01.2026.pdf", "ART_8", "APPOINTMENT_RESULT"},
		{"/wp-content/uploads/Lista-interviu-04.02.2026.pdf", "ART_8", "APPOINTMENT_INVITATION"},
	}

	for i, tt := range tests {
		if !strings.HasSuffix(links[i].URL, tt.url) {
			t.Errorf("link[%d].URL = %s, want suffix %s", i, links[i].URL, tt.url)
		}
		if links[i].Category != tt.category {
			t.Errorf("link[%d].Category = %s, want %s", i, links[i].Category, tt.category)
		}
		if links[i].Type != tt.fileType {
			t.Errorf("link[%d].Type = %s, want %s", i, links[i].Type, tt.fileType)
		}
	}
}

func TestParsePDFLinksFallback(t *testing.T) {
	parser := NewHTMLParser("https://cetatenie.just.ro")

	// Test fallback when no tabs found
	html := `
<!DOCTYPE html>
<html>
<body>
<div class="content">
	<a href="https://cetatenie.just.ro/wp-content/uploads/Art-11-2025.pdf">Art 11</a>
</div>
</body>
</html>
`

	links, err := parser.ParsePDFLinks(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParsePDFLinks() error = %v", err)
	}

	if len(links) != 1 {
		t.Fatalf("ParsePDFLinks() got %d links, want 1", len(links))
	}

	if links[0].Category != "ART_11" {
		t.Errorf("link.Category = %s, want ART_11", links[0].Category)
	}
}

func TestResolveURL(t *testing.T) {
	parser := NewHTMLParser("https://cetatenie.just.ro")

	tests := []struct {
		name string
		href string
		want string
	}{
		{
			name: "absolute URL",
			href: "https://other.com/file.pdf",
			want: "https://other.com/file.pdf",
		},
		{
			name: "relative URL with leading slash",
			href: "/wp-content/uploads/file.pdf",
			want: "https://cetatenie.just.ro/wp-content/uploads/file.pdf",
		},
		{
			name: "relative URL without leading slash",
			href: "wp-content/uploads/file.pdf",
			want: "https://cetatenie.just.ro/wp-content/uploads/file.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.resolveURL(tt.href)
			if got != tt.want {
				t.Errorf("resolveURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractCategoryFromContext(t *testing.T) {
	parser := NewHTMLParser("https://cetatenie.just.ro")

	tests := []struct {
		name    string
		context string
		want    string
	}{
		{"articolul-11-tab", "articolul-11-tab", "ART_11"},
		{"articolul-8-tab", "articolul-8-tab", "ART_8"},
		{"articolul-81-tab", "articolul-81-tab", "ART_8_1"},
		{"articolul-82-tab", "articolul-82-tab", "ART_8_2"},
		{"articolul-10-tab", "articolul-10-tab", "ART_10"},
		{"invitatii-interviu-art-8-tab", "invitatii-interviu-art-8-tab", "ART_8"},
		{"rezultate-interviu-art-8-tab", "rezultate-interviu-art-8-tab", "ART_8"},
		{"unknown", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.extractCategoryFromContext(tt.context, "", "")
			if got != tt.want {
				t.Errorf("extractCategoryFromContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractFileTypeFromContext(t *testing.T) {
	parser := NewHTMLParser("https://cetatenie.just.ro")

	tests := []struct {
		name    string
		context string
		want    string
	}{
		{"result tab", "rezultate-interviu-art-8-tab", "APPOINTMENT_RESULT"},
		{"invitation tab", "invitatii-interviu-art-8-tab", "APPOINTMENT_INVITATION"},
		{"application tab", "articolul-8-tab", "APPLICATION"},
		{"lista interviu", "lista interviu", "APPOINTMENT_INVITATION"},
		{"rezultate", "rezultate interviu 2026", "APPOINTMENT_RESULT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.extractFileTypeFromContext(tt.context, "", "")
			if got != tt.want {
				t.Errorf("extractFileTypeFromContext() = %v, want %v", got, tt.want)
			}
		})
	}
}
