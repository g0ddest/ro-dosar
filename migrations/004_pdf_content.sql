-- Table for storing PDF content to avoid passing between workflows
CREATE TABLE IF NOT EXISTS pdf_content (
    hash TEXT PRIMARY KEY,
    content BYTEA NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for cleanup of old content
CREATE INDEX IF NOT EXISTS idx_pdf_content_created_at ON pdf_content(created_at);
