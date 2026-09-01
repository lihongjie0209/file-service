ALTER TABLE files ADD COLUMN application_id TEXT NULL;
CREATE INDEX files_application_owner_idx ON files(tenant_id,application_id,owner_id,created_at DESC);
