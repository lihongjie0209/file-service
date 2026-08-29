package file

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/lihongjie0209/file-service/internal/database"
	"github.com/lihongjie0209/file-service/internal/objectstorage"
	"github.com/lihongjie0209/file-service/internal/principal"
)

type fakeRepository struct {
	file   Metadata
	outbox []OutboxEvent
}

func (f *fakeRepository) AddOutbox(_ context.Context, _ sqlx.ExtContext, event OutboxEvent) error {
	f.outbox = append(f.outbox, event)
	return nil
}

func (f *fakeRepository) Get(context.Context, string, string) (Metadata, error) {
	if f.file.ID == "" {
		return Metadata{}, ErrNotFound
	}
	return f.file, nil
}
func (f *fakeRepository) GetByKey(context.Context, string, string) (Metadata, error) {
	if f.file.ID == "" {
		return Metadata{}, ErrNotFound
	}
	return f.file, nil
}
func (f *fakeRepository) Insert(_ context.Context, _ sqlx.ExtContext, v Metadata) error {
	f.file = v
	return nil
}
func (f *fakeRepository) Update(_ context.Context, _ sqlx.ExtContext, v Metadata, expected int64) error {
	if f.file.Version != expected {
		return ErrStaleVersion
	}
	v.Version++
	f.file = v
	return nil
}

type fakeStorage struct {
	info    objectstorage.ObjectInfo
	deletes int
}

func (*fakeStorage) PresignUpload(context.Context, string, string, int64, string) (*url.URL, error) {
	return url.Parse("https://storage/upload")
}
func (*fakeStorage) PresignDownload(context.Context, string) (*url.URL, error) {
	return url.Parse("https://storage/download")
}
func (s *fakeStorage) Stat(context.Context, string) (objectstorage.ObjectInfo, error) {
	return s.info, nil
}
func (s *fakeStorage) Delete(context.Context, string) error { s.deletes++; return nil }
func (*fakeStorage) Bucket() string                         { return "files" }
func (*fakeStorage) TTL() time.Duration                     { return 15 * time.Minute }
func (*fakeStorage) MaxUploadBytes() int64                  { return 1024 }
func (*fakeStorage) Enabled() bool                          { return true }
func TestUploadLifecycleAndIdempotency(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	checksum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	repo := &fakeRepository{}
	storage := &fakeStorage{info: objectstorage.ObjectInfo{Size: 10, ChecksumSHA256: checksum}}
	service := NewService(repo, database.NewTransactor(sqlx.NewDb(db, "sqlmock")), storage)
	now := time.Date(2026, 8, 30, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	service.now = func() time.Time { return now }
	ctx := principal.WithContext(t.Context(), principal.Principal{Subject: "user-1"})
	mock.ExpectBegin()
	mock.ExpectCommit()
	first, err := service.InitiateUpload(ctx, "tenant-1", "../avatar.png", "image/png", 10, checksum, "upload-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.outbox) != 1 || repo.outbox[0].Subject != "platform.file.status.changed.v1" {
		t.Fatalf("outbox=%+v", repo.outbox)
	}
	replay, err := service.InitiateUpload(ctx, "tenant-1", "avatar.png", "image/png", 10, checksum, "upload-1")
	if err != nil || replay.File.ID != first.File.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	mock.ExpectBegin()
	mock.ExpectCommit()
	completed, err := service.CompleteUpload(ctx, first.File.ID, "tenant-1", checksum, 1)
	if err != nil || completed.Status != "ready" || completed.ScanStatus != "skipped" || completed.Version != 2 {
		t.Fatalf("completed=%+v err=%v", completed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRequiredScanBlocksDownloadUntilCleanResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := &fakeRepository{file: Metadata{ID: "file-1", TenantID: "tenant-1", ObjectKey: "key", Status: "ready", ScanStatus: "pending", Version: 2}}
	service := NewService(repository, database.NewTransactor(sqlx.NewDb(db, "sqlmock")), &fakeStorage{})
	service.scanRequired = true
	ctx := principal.WithContext(t.Context(), principal.Principal{Subject: "scanner-1"})
	if _, err := service.AuthorizeDownload(ctx, "file-1", "tenant-1"); err == nil {
		t.Fatal("download authorized before clean scan")
	}
	mock.ExpectBegin()
	mock.ExpectCommit()
	clean, err := service.ReportScanResult(ctx, "file-1", "tenant-1", "clean", 2)
	if err != nil || clean.ScanStatus != "clean" || clean.Version != 3 {
		t.Fatalf("clean=%+v err=%v", clean, err)
	}
	if _, err := service.AuthorizeDownload(ctx, "file-1", "tenant-1"); err != nil {
		t.Fatalf("download after clean scan: %v", err)
	}
}
func TestInitiateRejectsInvalidChecksum(t *testing.T) {
	service := NewService(&fakeRepository{}, &database.Transactor{}, &fakeStorage{})
	ctx := principal.WithContext(t.Context(), principal.Principal{Subject: "user"})
	if _, err := service.InitiateUpload(ctx, "tenant", "a.txt", "text/plain", 1, "bad", "key"); err == nil {
		t.Fatal("invalid checksum accepted")
	}
}

func TestDeleteRejectsMissingPrincipalBeforeStorageSideEffect(t *testing.T) {
	storage := &fakeStorage{}
	service := NewService(&fakeRepository{file: Metadata{ID: "file-1", TenantID: "tenant-1", ObjectKey: "key", Version: 1}}, &database.Transactor{}, storage)
	if _, err := service.Delete(t.Context(), "file-1", "tenant-1", 1); err == nil {
		t.Fatal("Delete() accepted missing principal")
	}
	if storage.deletes != 0 {
		t.Fatalf("storage delete calls = %d, want 0", storage.deletes)
	}
}
