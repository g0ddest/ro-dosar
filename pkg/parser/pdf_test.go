package parser

import (
	"testing"
	"time"

	"ro-dosar/internal/domain"
)

func TestParseDateFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     *time.Time
	}{
		{
			name:     "date in filename",
			filename: "Rezultate-interviu-27.01.2026.pdf",
			want:     timePtr(time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC)),
		},
		{
			name:     "date with full path",
			filename: "https://example.com/wp-content/uploads/2026/01/Lista-interviu-04.02.2026-1.pdf",
			want:     timePtr(time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC)),
		},
		{
			name:     "no date in filename",
			filename: "document.pdf",
			want:     nil,
		},
		{
			name:     "date with slash separator",
			filename: "results-15/03/2025.pdf",
			want:     timePtr(time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDateFromFilename(tt.filename)
			if tt.want == nil {
				if got != nil {
					t.Errorf("ParseDateFromFilename() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("ParseDateFromFilename() = nil, want %v", tt.want)
				return
			}
			if !got.Equal(*tt.want) {
				t.Errorf("ParseDateFromFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseApplicationsFromText(t *testing.T) {
	parser := NewPDFParser()

	text := `
NR. DOSAR    DATA ÎNREGISTRĂRII    TERMEN
10435/A/2025    15.01.2025    15.07.2025
10436/A/2025    16.01.2025    16.07.2025    215/P/2025
`

	records, err := parser.ParseApplicationsFromText(text, domain.CategoryArt8)
	if err != nil {
		t.Fatalf("ParseApplicationsFromText() error = %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("ParseApplicationsFromText() got %d records, want 2", len(records))
	}

	// Check first record
	if records[0].DocumentNumber.Number != 10435 {
		t.Errorf("record[0].DocumentNumber.Number = %d, want 10435", records[0].DocumentNumber.Number)
	}
	if records[0].DocumentNumber.Category != "A" {
		t.Errorf("record[0].DocumentNumber.Category = %s, want A", records[0].DocumentNumber.Category)
	}
	if records[0].RegisteredAt.Day() != 15 || records[0].RegisteredAt.Month() != 1 {
		t.Errorf("record[0].RegisteredAt = %v, want 15.01.2025", records[0].RegisteredAt)
	}
	if records[0].Term == nil {
		t.Error("record[0].Term should not be nil")
	}

	// Check second record has solution number
	if records[1].SolutionNumber == nil {
		t.Error("record[1].SolutionNumber should not be nil")
	} else if records[1].SolutionNumber.Number != 215 {
		t.Errorf("record[1].SolutionNumber.Number = %d, want 215", records[1].SolutionNumber.Number)
	}
}

func TestParseAppointmentsFromText(t *testing.T) {
	parser := NewPDFParser()
	appointmentDate := time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC)

	t.Run("invitation records", func(t *testing.T) {
		text := `
Lista persoanelor invitate
10435/A/2025    09:00
10436/A/2025    09:30
`
		records, err := parser.ParseAppointmentsFromText(text, domain.AppointmentTypeInvitation, &appointmentDate)
		if err != nil {
			t.Fatalf("ParseAppointmentsFromText() error = %v", err)
		}

		if len(records) != 2 {
			t.Fatalf("ParseAppointmentsFromText() got %d records, want 2", len(records))
		}

		if records[0].Date != appointmentDate {
			t.Errorf("record[0].Date = %v, want %v", records[0].Date, appointmentDate)
		}
		if records[0].Time == nil {
			t.Error("record[0].Time should not be nil")
		} else if records[0].Time.Hour() != 9 || records[0].Time.Minute() != 0 {
			t.Errorf("record[0].Time = %v, want 09:00", records[0].Time)
		}
	})

	t.Run("result records", func(t *testing.T) {
		text := `
Rezultate interviu 27.01.2026
10435/A/2025    Aviz pozitiv
10436/A/2025    Absent
10437/A/2025    Amânare
`
		records, err := parser.ParseAppointmentsFromText(text, domain.AppointmentTypeResult, &appointmentDate)
		if err != nil {
			t.Fatalf("ParseAppointmentsFromText() error = %v", err)
		}

		if len(records) != 3 {
			t.Fatalf("ParseAppointmentsFromText() got %d records, want 3", len(records))
		}

		if records[0].Result == nil || *records[0].Result != "Aviz pozitiv" {
			t.Errorf("record[0].Result = %v, want 'Aviz pozitiv'", records[0].Result)
		}
		if records[1].Result == nil || *records[1].Result != "Absent" {
			t.Errorf("record[1].Result = %v, want 'Absent'", records[1].Result)
		}
		if records[2].Result == nil || *records[2].Result != "Amânare" {
			t.Errorf("record[2].Result = %v, want 'Amânare'", records[2].Result)
		}
	})

	t.Run("extract date from text when not provided", func(t *testing.T) {
		text := `
Rezultate interviu 15.03.2025
10435/A/2025    Aviz pozitiv
`
		records, err := parser.ParseAppointmentsFromText(text, domain.AppointmentTypeResult, nil)
		if err != nil {
			t.Fatalf("ParseAppointmentsFromText() error = %v", err)
		}

		if len(records) != 1 {
			t.Fatalf("ParseAppointmentsFromText() got %d records, want 1", len(records))
		}

		expectedDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
		if !records[0].Date.Equal(expectedDate) {
			t.Errorf("record[0].Date = %v, want %v", records[0].Date, expectedDate)
		}
	})

	t.Run("result with Termen format", func(t *testing.T) {
		text := `
Rezultate interviu 18.07.2023
14.Dosar nr.27783/A/2021 - Termen 17.10.2023
15.Dosar nr.27784/A/2021 - Aviz pozitiv
`
		records, err := parser.ParseAppointmentsFromText(text, domain.AppointmentTypeResult, nil)
		if err != nil {
			t.Fatalf("ParseAppointmentsFromText() error = %v", err)
		}

		if len(records) != 2 {
			t.Fatalf("ParseAppointmentsFromText() got %d records, want 2", len(records))
		}

		// First record should have Termen result
		if records[0].Result == nil {
			t.Error("record[0].Result should not be nil")
		} else if *records[0].Result != "Termen 17.10.2023" {
			t.Errorf("record[0].Result = %q, want 'Termen 17.10.2023'", *records[0].Result)
		}

		// Second record should have Aviz pozitiv
		if records[1].Result == nil || *records[1].Result != "Aviz pozitiv" {
			t.Errorf("record[1].Result = %v, want 'Aviz pozitiv'", records[1].Result)
		}
	})
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestParseOathList(t *testing.T) {
	text := `              TABEL CU DOSARELE PERSOANELOR CARE URMEAZĂ
SA DEPUNĂ JURĂMÂNTUL DE CREDINTA FATĂ DE ROMÂNIA LA DATA DE 12.08.2026
                          ORA 09.00

                                      NR DOSAR
          NR CRT
              1.                      35573/2022
              2.                      28886/2022
              3.                      32914/2021
              4.                      32914/2021
              5.                       7921/2024
`

	list, err := ParseOathList(text)
	if err != nil {
		t.Fatal(err)
	}
	if !list.Date.Equal(time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("wrong date: %v", list.Date)
	}
	if list.Time == nil || *list.Time != "09:00" {
		t.Errorf("wrong time: %v", list.Time)
	}
	// duplicates (family members) deduplicated
	if len(list.Entries) != 4 {
		t.Fatalf("expected 4 unique entries, got %d: %+v", len(list.Entries), list.Entries)
	}
	if list.Entries[0].Number != 35573 || list.Entries[0].Year != 2022 {
		t.Errorf("wrong first entry: %+v", list.Entries[0])
	}
	if list.Entries[3].Number != 7921 || list.Entries[3].Year != 2024 {
		t.Errorf("wrong last entry: %+v", list.Entries[3])
	}
}

func TestParseOathList_HourVariantsAndMissing(t *testing.T) {
	list, err := ParseOathList("JURĂMÂNTUL ... LA DATA DE 5.09.2026\nORA 14:30\n1. 100/2020\n")
	if err != nil || list.Time == nil || *list.Time != "14:30" {
		t.Fatalf("hour variant failed: %+v %v", list, err)
	}

	list, err = ParseOathList("LA DATA DE 05.09.2026\n1. 100/2020\n")
	if err != nil || list.Time != nil {
		t.Fatalf("missing hour must yield nil time: %+v %v", list, err)
	}

	if _, err = ParseOathList("no date here\n1. 100/2020\n"); err == nil {
		t.Fatal("missing date must error")
	}
}
