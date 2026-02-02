-- Migration: Create documents table
-- Description: Stores citizenship application documents

CREATE TABLE IF NOT EXISTS documents (
    document_number TEXT PRIMARY KEY,         -- normalized: 10435/A/2025
    registered_at   DATE NOT NULL,
    category        TEXT NOT NULL,            -- ART_8, ART_8_1, ART_8_2, ART_10, ART_11
    term            DATE,
    solution_number TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Index for category queries
CREATE INDEX IF NOT EXISTS idx_documents_category ON documents(category);

-- Index for term queries
CREATE INDEX IF NOT EXISTS idx_documents_term ON documents(term) WHERE term IS NOT NULL;

-- Index for solution_number queries
CREATE INDEX IF NOT EXISTS idx_documents_solution ON documents(solution_number) WHERE solution_number IS NOT NULL;
