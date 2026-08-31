CREATE INDEX file_outbox_retention_idx ON file_outbox_events (published_at, id) WHERE published_at IS NOT NULL;
