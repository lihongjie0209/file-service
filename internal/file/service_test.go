package file

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/lihongjie0209/file-service/internal/database"
	"github.com/lihongjie0209/file-service/internal/objectstorage"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
)

type fakeRepository struct {
	file    Metadata
	outbox  []OutboxEvent
	expired []Metadata
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
func (f *fakeRepository) List(context.Context, ListFilter) ([]Metadata, int64, error) {
	if f.file.ID == "" {
		return []Metadata{}, 0, nil
	}
	return []Metadata{f.file}, 1, nil
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
func (f *fakeRepository) ListExpiredUploads(context.Context, time.Time, int) ([]Metadata, error) {
	return f.expired, nil
}

type fakeStorage struct {
	info             objectstorage.ObjectInfo
	deletes          int
	deleteErr        error
	completedParts   []objectstorage.CompletedPart
	multipartAborted int
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
func (s *fakeStorage) Delete(context.Context, string) error { s.deletes++; return s.deleteErr }
func (*fakeStorage) InitiateMultipart(context.Context, string, string, string) (string, error) {
	return "upload-1", nil
}
func (*fakeStorage) PresignUploadPart(context.Context, string, string, int) (*url.URL, error) {
	return url.Parse("https://storage/upload-part")
}

func (s *fakeStorage) CompleteMultipart(_ context.Context, _ string, _ string, parts []objectstorage.CompletedPart) error {
	s.completedParts = append([]objectstorage.CompletedPart(nil), parts...)
	return nil
}
func (s *fakeStorage) AbortMultipart(context.Context, string, string) error {
	s.multipartAborted++
	return nil
}
func (*fakeStorage) Bucket() string        { return "files" }
func (*fakeStorage) TTL() time.Duration    { return 15 * time.Minute }
func (*fakeStorage) MaxUploadBytes() int64 { return 1 << 30 }
func (*fakeStorage) Enabled() bool         { return true }
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
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
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
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "scanner-1", Type: platformprincipal.TypeServiceAccount})
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
func TestMultipartUploadLifecycle(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	checksum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	repository := &fakeRepository{}
	storage := &fakeStorage{info: objectstorage.ObjectInfo{Size: 11 << 20, ChecksumSHA256: checksum}}
	service := NewService(repository, database.NewTransactor(sqlx.NewDb(db, "sqlmock")), storage)
	now := time.Date(2026, 8, 30, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	service.now = func() time.Time { return now }
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	mock.ExpectBegin()
	mock.ExpectCommit()
	initiated, err := service.InitiateMultipartUpload(ctx, "tenant-1", "archive.bin", "application/octet-stream", 11<<20, checksum, "multipart-1", 5<<20)
	if err != nil || initiated.PartCount != 3 || initiated.UploadID != "upload-1" || initiated.File.UploadMode != "multipart" {
		t.Fatalf("initiated=%+v err=%v", initiated, err)
	}
	part, err := service.AuthorizeUploadPart(ctx, initiated.File.ID, "tenant-1", 2)
	if err != nil || part.URL == "" {
		t.Fatalf("part=%+v err=%v", part, err)
	}
	mock.ExpectBegin()
	mock.ExpectCommit()
	completed, err := service.CompleteMultipartUpload(ctx, initiated.File.ID, "tenant-1", checksum, []CompletedPart{{PartNumber: 3, ETag: "c"}, {PartNumber: 1, ETag: `"a"`}, {PartNumber: 2, ETag: "b"}}, initiated.File.Version)
	if err != nil || completed.Status != "ready" || len(storage.completedParts) != 3 || storage.completedParts[0].PartNumber != 1 {
		t.Fatalf("completed=%+v parts=%+v err=%v", completed, storage.completedParts, err)
	}
}
func TestInitiateRejectsInvalidChecksum(t *testing.T) {
	service := NewService(&fakeRepository{}, &database.Transactor{}, &fakeStorage{})
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user", Type: platformprincipal.TypeUser, TenantID: "tenant"})
	if _, err := service.InitiateUpload(ctx, "tenant", "a.txt", "text/plain", 1, "bad", "key"); err == nil {
		t.Fatal("invalid checksum accepted")
	}
}

func TestListRejectsCrossTenantRequest(t *testing.T) {
	service := NewService(&fakeRepository{}, &database.Transactor{}, &fakeStorage{})
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	if _, err := service.List(ctx, ListFilter{TenantID: "tenant-2"}); err == nil {
		t.Fatal("List() accepted a cross-tenant request")
	}
}

func TestListAppliesPaginationDefaults(t *testing.T) {
	repository := &fakeRepository{file: Metadata{ID: "file-1", TenantID: "tenant-1"}}
	service := NewService(repository, &database.Transactor{}, &fakeStorage{})
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	page, err := service.List(ctx, ListFilter{TenantID: "tenant-1"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Page != 1 || page.PageSize != 20 || len(page.Files) != 1 {
		t.Fatalf("page = %+v", page)
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

func TestDeletePersistsDeletingBeforeObjectStorageSideEffectAndCanRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := &fakeRepository{file: Metadata{ID: "file-1", TenantID: "tenant-1", ObjectKey: "key", Status: "ready", Version: 1}}
	storage := &fakeStorage{deleteErr: errors.New("temporary outage")}
	service := NewService(repository, database.NewTransactor(sqlx.NewDb(db, "sqlmock")), storage)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	mock.ExpectBegin()
	mock.ExpectCommit()
	deleting, err := service.Delete(ctx, "file-1", "tenant-1", 1)
	if err == nil || deleting.Status != "deleting" || deleting.Version != 2 || repository.file.Status != "deleting" {
		t.Fatalf("deleting=%+v stored=%+v err=%v", deleting, repository.file, err)
	}
	storage.deleteErr = nil
	mock.ExpectBegin()
	mock.ExpectCommit()
	deleted, err := service.Delete(ctx, "file-1", "tenant-1", deleting.Version)
	if err != nil || deleted.Status != "deleted" || deleted.Version != 3 || storage.deletes != 2 {
		t.Fatalf("deleted=%+v calls=%d err=%v", deleted, storage.deletes, err)
	}
}

func TestCleanupExpiredMultipartUpload(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 8, 30, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	expiresAt := now.Add(-time.Minute)
	value := Metadata{ID: "file-1", TenantID: "tenant-1", ObjectKey: "key", UploadMode: "multipart", MultipartUploadID: "upload-1", Status: "pending_upload", UploadExpiresAt: &expiresAt, Version: 1, CreatedAt: now, UpdatedAt: now}
	repository := &fakeRepository{file: value, expired: []Metadata{value}}
	storage := &fakeStorage{}
	service := NewService(repository, database.NewTransactor(sqlx.NewDb(db, "sqlmock")), storage)
	service.now = func() time.Time { return now }
	mock.ExpectBegin()
	mock.ExpectCommit()
	count, err := service.CleanupExpiredUploads(t.Context())
	if err != nil || count != 1 || storage.multipartAborted != 1 || repository.file.Status != "expired" || repository.file.Version != 2 {
		t.Fatalf("count=%d aborts=%d file=%+v err=%v", count, storage.multipartAborted, repository.file, err)
	}
}
