ALTER TABLE files ADD COLUMN application_id VARCHAR(191) NULL AFTER tenant_id, ADD INDEX files_application_owner_idx(tenant_id,application_id,owner_id,created_at DESC);
