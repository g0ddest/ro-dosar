-- Function to clean up PDF content that has been processed
-- Deletes content where corresponding parsed_files entry exists
CREATE OR REPLACE FUNCTION cleanup_processed_pdf_content() RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    WITH deleted AS (
        DELETE FROM pdf_content pc
        WHERE EXISTS (
            SELECT 1 FROM parsed_files pf WHERE pf.hash = pc.hash
        )
        RETURNING 1
    )
    SELECT COUNT(*) INTO deleted_count FROM deleted;

    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up orphaned PDF content (content without parsed_files entry, older than 1 hour)
CREATE OR REPLACE FUNCTION cleanup_orphaned_pdf_content() RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    WITH deleted AS (
        DELETE FROM pdf_content pc
        WHERE NOT EXISTS (
            SELECT 1 FROM parsed_files pf WHERE pf.hash = pc.hash
        )
        AND pc.created_at < NOW() - INTERVAL '1 hour'
        RETURNING 1
    )
    SELECT COUNT(*) INTO deleted_count FROM deleted;

    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Combined cleanup function
CREATE OR REPLACE FUNCTION cleanup_pdf_content() RETURNS TABLE(processed INTEGER, orphaned INTEGER) AS $$
BEGIN
    RETURN QUERY
    SELECT
        cleanup_processed_pdf_content() as processed,
        cleanup_orphaned_pdf_content() as orphaned;
END;
$$ LANGUAGE plpgsql;

-- Schedule cleanup to run every hour (requires pg_cron extension)
-- To enable: CREATE EXTENSION IF NOT EXISTS pg_cron;
-- Then run: SELECT cron.schedule('cleanup-pdf-content', '0 * * * *', 'SELECT cleanup_pdf_content()');
--
-- For docker, add to docker-compose.yml postgres service:
--   command: postgres -c shared_preload_libraries=pg_cron -c cron.database_name=dosar
