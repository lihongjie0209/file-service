ALTER TABLE files DROP INDEX files_expired_upload_idx;
ALTER TABLE files DROP COLUMN upload_expires_at, DROP COLUMN part_count, DROP COLUMN part_size, DROP COLUMN multipart_upload_id, DROP COLUMN upload_mode;
