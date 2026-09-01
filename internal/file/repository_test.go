package file

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestRepositoryGetFiltersTenantAndApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(sqlx.NewDb(db, "sqlmock"))
	query := `SELECT ` + columns + ` FROM files WHERE id=? AND tenant_id=? AND application_id=?`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("file-1", "tenant-1", "app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "application_id", "owner_id", "bucket", "object_key", "filename", "content_type", "size", "checksum_sha256", "status", "scan_status", "idempotency_key", "upload_mode", "multipart_upload_id", "part_size", "part_count", "upload_expires_at", "version", "created_at", "updated_at", "created_by", "updated_by"}).
			AddRow("file-1", "tenant-1", "app-1", "user-1", "files", "key", "a.txt", "text/plain", 1, "checksum", "ready", "skipped", "key-1", "single", "", 0, 0, nil, 1, time.Now(), time.Now(), "user-1", "user-1"))
	value, err := repository.Get(t.Context(), "file-1", "tenant-1", "app-1")
	if err != nil {
		t.Fatal(err)
	}
	if value.ApplicationID != "app-1" {
		t.Fatalf("application_id = %q", value.ApplicationID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
