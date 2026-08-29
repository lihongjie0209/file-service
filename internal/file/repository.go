package file

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("file not found")
var ErrStaleVersion = errors.New("stale file version")

type Repository interface {
	Get(context.Context, string, string) (Metadata, error)
	GetByKey(context.Context, string, string) (Metadata, error)
	Insert(context.Context, sqlx.ExtContext, Metadata) error
	Update(context.Context, sqlx.ExtContext, Metadata, int64) error
	AddOutbox(context.Context, sqlx.ExtContext, OutboxEvent) error
}

func (r *SQLRepository) AddOutbox(ctx context.Context, executor sqlx.ExtContext, event OutboxEvent) error {
	_, err := executor.ExecContext(ctx, r.db.Rebind(`INSERT INTO file_outbox_events (id,subject,envelope,attempts,available_at,published_at,last_error,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,0,?,NULL,'',1,?,?,?,?)`), event.ID, event.Subject, event.Envelope, event.AvailableAt, event.CreatedAt, event.UpdatedAt, event.CreatedBy, event.UpdatedBy)
	return err
}

type SQLRepository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &SQLRepository{db: db} }

const columns = `id,tenant_id,owner_id,bucket,object_key,filename,content_type,size,checksum_sha256,status,scan_status,idempotency_key,version,created_at,updated_at,created_by,updated_by`

func (r *SQLRepository) Get(ctx context.Context, id, tenant string) (Metadata, error) {
	var v Metadata
	err := r.db.GetContext(ctx, &v, r.db.Rebind(`SELECT `+columns+` FROM files WHERE id=? AND tenant_id=?`), id, tenant)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return v, err
}
func (r *SQLRepository) GetByKey(ctx context.Context, tenant, key string) (Metadata, error) {
	var v Metadata
	err := r.db.GetContext(ctx, &v, r.db.Rebind(`SELECT `+columns+` FROM files WHERE tenant_id=? AND idempotency_key=?`), tenant, key)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return v, err
}
func (r *SQLRepository) Insert(ctx context.Context, e sqlx.ExtContext, v Metadata) error {
	_, err := e.ExecContext(ctx, r.db.Rebind(`INSERT INTO files (`+columns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), v.ID, v.TenantID, v.OwnerID, v.Bucket, v.ObjectKey, v.Filename, v.ContentType, v.Size, v.ChecksumSHA256, v.Status, v.ScanStatus, v.IdempotencyKey, v.Version, v.CreatedAt, v.UpdatedAt, v.CreatedBy, v.UpdatedBy)
	return err
}
func (r *SQLRepository) Update(ctx context.Context, e sqlx.ExtContext, v Metadata, expected int64) error {
	result, err := e.ExecContext(ctx, r.db.Rebind(`UPDATE files SET status=?,scan_status=?,checksum_sha256=?,size=?,version=version+1,updated_at=?,updated_by=? WHERE id=? AND tenant_id=? AND version=?`), v.Status, v.ScanStatus, v.ChecksumSHA256, v.Size, v.UpdatedAt, v.UpdatedBy, v.ID, v.TenantID, expected)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrStaleVersion
	}
	return err
}
