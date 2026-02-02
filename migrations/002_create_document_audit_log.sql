-- Migration: Create document audit log table
-- Description: Stores audit trail for document changes

CREATE TABLE IF NOT EXISTS document_audit_log (
    id              SERIAL PRIMARY KEY,
    document_number TEXT NOT NULL,
    action          TEXT NOT NULL,            -- CREATE, UPDATE, DELETE
    old_state       JSONB,
    new_state       JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Index for document queries
CREATE INDEX IF NOT EXISTS idx_audit_document_number ON document_audit_log(document_number);

-- Index for time-based queries
CREATE INDEX IF NOT EXISTS idx_audit_created_at ON document_audit_log(created_at);
