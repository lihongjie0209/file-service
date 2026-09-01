CREATE INDEX files_tenant_owner_idx ON files(tenant_id,owner_id,created_at DESC);
ALTER TABLE files DROP CONSTRAINT files_scope_idempotency_key;
ALTER TABLE files ADD CONSTRAINT files_tenant_id_idempotency_key_key UNIQUE(tenant_id,idempotency_key);
ALTER TABLE files DROP CONSTRAINT chk_files_application_nonempty;
ALTER TABLE files ALTER COLUMN application_id DROP NOT NULL;
