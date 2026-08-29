ALTER TABLE files
    ADD COLUMN upload_mode TEXT NULL,
    ADD COLUMN multipart_upload_id TEXT NULL,
    ADD COLUMN part_size BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN part_count INT NOT NULL DEFAULT 0,
    ADD COLUMN upload_expires_at DATETIME(6) NULL;

UPDATE files SET upload_mode = 'single', multipart_upload_id = '';
ALTER TABLE files
    MODIFY upload_mode TEXT NOT NULL,
    MODIFY multipart_upload_id TEXT NOT NULL,
    ADD INDEX files_expired_upload_idx (status(32), upload_expires_at, id);
