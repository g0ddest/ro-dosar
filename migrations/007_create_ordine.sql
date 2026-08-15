-- Migration: Create ordine table
-- Description: Index of published ANC ordins (number, date, PDF url) parsed
-- from the Ordine listing pages; the PDFs themselves are never downloaded

CREATE TABLE IF NOT EXISTS ordine (
    url          TEXT PRIMARY KEY,
    ordin_number INTEGER NOT NULL,
    letter       TEXT NOT NULL,
    ordin_date   DATE NOT NULL,
    source_page  TEXT NOT NULL,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ordine_number ON ordine(ordin_number, letter);
