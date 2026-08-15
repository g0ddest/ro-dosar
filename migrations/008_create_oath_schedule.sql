-- Migration: Create oath_schedule table
-- Description: Dossiers scheduled for the citizenship oath at ANC Bucharest,
-- parsed from the /juramant/ listing PDFs (dossier numbers carry no category)

CREATE TABLE IF NOT EXISTS oath_schedule (
    doc_number  INTEGER NOT NULL,
    doc_year    INTEGER NOT NULL,
    oath_date   DATE NOT NULL,
    oath_time   TIME,
    list_url    TEXT NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (doc_number, doc_year, oath_date)
);

CREATE INDEX IF NOT EXISTS idx_oath_schedule_doc ON oath_schedule(doc_number, doc_year);
