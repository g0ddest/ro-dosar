-- Migration: Add status column to parsed_files table
-- Description: Tracks file status (PARSED, NOT_FOUND) to avoid retrying deleted files

ALTER TABLE parsed_files ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'PARSED';

-- Index for status queries (useful for finding NOT_FOUND files)
CREATE INDEX IF NOT EXISTS idx_parsed_files_status ON parsed_files(status);
