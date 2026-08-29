ALTER TABLE files
    ADD COLUMN upload_mode TEXT NOT NULL DEFAULT 'single',
    ADD COLUMN multipart_upload_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN part_size BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN part_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN upload_expires_at TIMESTAMPTZ NULL;

CREATE INDEX files_expired_upload_idx ON files (upload_expires_at, id)
    WHERE status = 'pending_upload' AND upload_expires_at IS NOT NULL;
