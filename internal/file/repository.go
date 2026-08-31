package file

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("file not found")
var ErrStaleVersion = errors.New("stale file version")

type Repository interface {
	Get(context.Context, string, string) (Metadata, error)
	GetByKey(context.Context, string, string) (Metadata, error)
	List(context.Context, ListFilter) ([]Metadata, int64, error)
	Insert(context.Context, sqlx.ExtContext, Metadata) error
	Update(context.Context, sqlx.ExtContext, Metadata, int64) error
	ListExpiredUploads(context.Context, time.Time, int) ([]Metadata, error)
	AddOutbox(context.Context, sqlx.ExtContext, OutboxEvent) error
}

func (r *SQLRepository) AddOutbox(ctx context.Context, executor sqlx.ExtContext, event OutboxEvent) error {
	_, err := executor.ExecContext(ctx, r.db.Rebind(`INSERT INTO file_outbox_events (id,subject,envelope,attempts,available_at,published_at,last_error,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,0,?,NULL,'',1,?,?,?,?)`), event.ID, event.Subject, event.Envelope, event.AvailableAt, event.CreatedAt, event.UpdatedAt, event.CreatedBy, event.UpdatedBy)
	return err
}

type SQLRepository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &SQLRepository{db: db} }

const columns = `id,tenant_id,owner_id,bucket,object_key,filename,content_type,size,checksum_sha256,status,scan_status,idempotency_key,upload_mode,multipart_upload_id,part_size,part_count,upload_expires_at,version,created_at,updated_at,created_by,updated_by`

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
func (r *SQLRepository) List(ctx context.Context, filter ListFilter) ([]Metadata, int64, error) {
	where := ` WHERE tenant_id=?`
	args := []any{filter.TenantID}
	if filter.Keyword != "" {
		where += ` AND (LOWER(filename) LIKE ? OR LOWER(id) LIKE ?)`
		keyword := "%" + strings.ToLower(filter.Keyword) + "%"
		args = append(args, keyword, keyword)
	}
	for _, item := range []struct {
		column string
		value  string
	}{{"status", filter.Status}, {"scan_status", filter.ScanStatus}, {"content_type", filter.ContentType}, {"owner_id", filter.OwnerID}} {
		if item.value != "" {
			where += ` AND ` + item.column + `=?`
			args = append(args, item.value)
		}
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(`SELECT COUNT(*) FROM files`+where), args...); err != nil {
		return nil, 0, err
	}
	values := make([]Metadata, 0)
	listArgs := append(append([]any(nil), args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	err := r.db.SelectContext(ctx, &values, r.db.Rebind(`SELECT `+columns+` FROM files`+where+` ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`), listArgs...)
	return values, total, err
}
func (r *SQLRepository) Insert(ctx context.Context, e sqlx.ExtContext, v Metadata) error {
	_, err := e.ExecContext(ctx, r.db.Rebind(`INSERT INTO files (`+columns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), v.ID, v.TenantID, v.OwnerID, v.Bucket, v.ObjectKey, v.Filename, v.ContentType, v.Size, v.ChecksumSHA256, v.Status, v.ScanStatus, v.IdempotencyKey, v.UploadMode, v.MultipartUploadID, v.PartSize, v.PartCount, v.UploadExpiresAt, v.Version, v.CreatedAt, v.UpdatedAt, v.CreatedBy, v.UpdatedBy)
	return err
}
func (r *SQLRepository) Update(ctx context.Context, e sqlx.ExtContext, v Metadata, expected int64) error {
	result, err := e.ExecContext(ctx, r.db.Rebind(`UPDATE files SET status=?,scan_status=?,checksum_sha256=?,size=?,upload_mode=?,multipart_upload_id=?,part_size=?,part_count=?,upload_expires_at=?,version=version+1,updated_at=?,updated_by=? WHERE id=? AND tenant_id=? AND version=?`), v.Status, v.ScanStatus, v.ChecksumSHA256, v.Size, v.UploadMode, v.MultipartUploadID, v.PartSize, v.PartCount, v.UploadExpiresAt, v.UpdatedAt, v.UpdatedBy, v.ID, v.TenantID, expected)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrStaleVersion
	}
	return err
}
func (r *SQLRepository) ListExpiredUploads(ctx context.Context, before time.Time, limit int) ([]Metadata, error) {
	var values []Metadata
	err := r.db.SelectContext(ctx, &values, r.db.Rebind(`SELECT `+columns+` FROM files WHERE status='pending_upload' AND upload_expires_at IS NOT NULL AND upload_expires_at<=? ORDER BY upload_expires_at,id LIMIT ?`), before, limit)
	return values, err
}
