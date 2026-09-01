ALTER TABLE files ALTER COLUMN application_id SET NOT NULL;
ALTER TABLE files ADD CONSTRAINT chk_files_application_nonempty CHECK(application_id <> '');
ALTER TABLE files DROP CONSTRAINT files_tenant_id_idempotency_key_key;
ALTER TABLE files ADD CONSTRAINT files_scope_idempotency_key UNIQUE(tenant_id,application_id,idempotency_key);
DROP INDEX files_tenant_owner_idx;
