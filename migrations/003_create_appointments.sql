-- Migration: Create appointments table
-- Description: Stores appointment invitations and results

CREATE TABLE IF NOT EXISTS appointments (
    id              SERIAL PRIMARY KEY,
    document_number TEXT NOT NULL,
    date            DATE NOT NULL,
    time            TIME,
    result          TEXT,                     -- Aviz pozitiv, Absent, Amânare, etc.
    type            TEXT NOT NULL,            -- INVITATION, RESULT
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(document_number, date, type)
);

-- Index for document queries
CREATE INDEX IF NOT EXISTS idx_appointments_document ON appointments(document_number);

-- Index for date queries
CREATE INDEX IF NOT EXISTS idx_appointments_date ON appointments(date);

-- Index for type queries
CREATE INDEX IF NOT EXISTS idx_appointments_type ON appointments(type);
