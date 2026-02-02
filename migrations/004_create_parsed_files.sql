-- Migration: Create parsed files table
-- Description: Tracks parsed PDF files and their hashes for change detection

CREATE TABLE IF NOT EXISTS parsed_files (
    uri        TEXT PRIMARY KEY,
    hash       TEXT NOT NULL,                 -- SHA-256 hash
    category   TEXT NOT NULL,                 -- ART_8, ART_8_1, ART_8_2, ART_10, ART_11
    type       TEXT NOT NULL,                 -- APPLICATION, APPOINTMENT_INVITATION, APPOINTMENT_RESULT
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for category queries
CREATE INDEX IF NOT EXISTS idx_parsed_files_category ON parsed_files(category);

-- Index for type queries
CREATE INDEX IF NOT EXISTS idx_parsed_files_type ON parsed_files(type);
