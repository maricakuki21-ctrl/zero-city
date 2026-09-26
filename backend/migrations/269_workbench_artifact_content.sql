-- Historical metadata-only artifacts remain NULL rather than fabricated.
ALTER TABLE workbench_artifacts
    ADD COLUMN IF NOT EXISTS content_bytes BYTEA;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'workbench_artifacts'::regclass
          AND conname = 'workbench_artifacts_content_size_check'
    ) THEN
        ALTER TABLE workbench_artifacts
            ADD CONSTRAINT workbench_artifacts_content_size_check
            CHECK (content_bytes IS NULL OR
                (octet_length(content_bytes) = byte_size AND octet_length(content_bytes) <= 4194304));
    END IF;
END;
$$;
