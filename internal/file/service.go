package file

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lihongjie0209/file-service/internal/apperror"
	"github.com/lihongjie0209/file-service/internal/config"
	"github.com/lihongjie0209/file-service/internal/database"
	"github.com/lihongjie0209/file-service/internal/objectstorage"
	"github.com/lihongjie0209/file-service/internal/principal"
	"github.com/lihongjie0209/microservice-platform-go/eventbus"
	filev1 "github.com/lihongjie0209/platform-protos/gen/go/platform/file/v1"
	"go.uber.org/fx"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	repository   Repository
	transactor   *database.Transactor
	storage      objectstorage.Storage
	now          func() time.Time
	scanRequired bool
}

func NewService(r Repository, t *database.Transactor, s objectstorage.Storage) *Service {
	return &Service{repository: r, transactor: t, storage: s, now: time.Now}
}
func NewRuntimeService(r Repository, t *database.Transactor, s objectstorage.Storage, cfg config.Config) *Service {
	service := NewService(r, t, s)
	service.scanRequired = cfg.ObjectStorage.ScanRequired
	return service
}
func (s *Service) InitiateUpload(ctx context.Context, tenant, filename, contentType string, size int64, checksum, key string) (Authorization, error) {
	tenant = strings.TrimSpace(tenant)
	filename = filepath.Base(strings.TrimSpace(filename))
	contentType = strings.TrimSpace(contentType)
	checksum = strings.ToLower(strings.TrimSpace(checksum))
	key = strings.TrimSpace(key)
	if tenant == "" || filename == "" || filename == "." || key == "" || size <= 0 || s.storage == nil || !s.storage.Enabled() || size > s.storage.MaxUploadBytes() {
		return Authorization{}, apperror.Invalid("invalid upload request or object storage unavailable", nil)
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil {
		return Authorization{}, apperror.Invalid("invalid content_type", err)
	}
	if !validChecksum(checksum) {
		return Authorization{}, apperror.Invalid("checksum_sha256 must be 64 hexadecimal characters", nil)
	}
	caller, ok := principal.FromContext(ctx)
	if !ok {
		return Authorization{}, apperror.Unauthorized("authenticated actor is required")
	}
	if existing, err := s.repository.GetByKey(ctx, tenant, key); err == nil {
		urlValue, urlErr := s.storage.PresignUpload(ctx, existing.ObjectKey, existing.ContentType, existing.Size, existing.ChecksumSHA256)
		if urlErr != nil {
			return Authorization{}, apperror.Unavailable("object storage unavailable", urlErr)
		}
		return Authorization{File: existing, URL: urlValue.String(), Headers: uploadHeaders(existing.ContentType, existing.Size, existing.ChecksumSHA256), ExpiresAt: s.now().Add(s.storage.TTL())}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Authorization{}, translate(err)
	}
	now := s.now()
	id := uuid.NewString()
	objectKey := fmt.Sprintf("%s/%s/%s", tenant, id, filename)
	v := Metadata{ID: id, TenantID: tenant, OwnerID: caller.Subject, Bucket: s.storage.Bucket(), ObjectKey: objectKey, Filename: filename, ContentType: contentType, Size: size, ChecksumSHA256: checksum, Status: "pending_upload", ScanStatus: "pending", IdempotencyKey: key, Version: 1, CreatedAt: now, UpdatedAt: now, CreatedBy: caller.Subject, UpdatedBy: caller.Subject}
	err := s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if err := s.repository.Insert(ctx, tx, v); err != nil {
			return err
		}
		return s.addStatusEvent(ctx, tx, v)
	})
	if err != nil {
		return Authorization{}, translate(err)
	}
	urlValue, err := s.storage.PresignUpload(ctx, objectKey, contentType, size, checksum)
	if err != nil {
		return Authorization{}, apperror.Unavailable("object storage unavailable", err)
	}
	return Authorization{File: v, URL: urlValue.String(), Headers: uploadHeaders(contentType, size, checksum), ExpiresAt: now.Add(s.storage.TTL())}, nil
}
func uploadHeaders(contentType string, size int64, checksum string) map[string]string {
	return map[string]string{"Content-Type": contentType, "Content-Length": fmt.Sprint(size), "X-Amz-Meta-Sha256": checksum}
}
func (s *Service) CompleteUpload(ctx context.Context, id, tenant, checksum string, expected int64) (Metadata, error) {
	caller, ok := principal.FromContext(ctx)
	if !ok {
		return Metadata{}, apperror.Unauthorized("authenticated actor is required")
	}
	v, err := s.repository.Get(ctx, id, tenant)
	if err != nil {
		return Metadata{}, translate(err)
	}
	if v.Status == "ready" {
		return v, nil
	}
	if expected < 1 {
		return Metadata{}, apperror.Invalid("expected_version is required", nil)
	}
	info, err := s.storage.Stat(ctx, v.ObjectKey)
	if err != nil {
		return Metadata{}, apperror.Unavailable("uploaded object is unavailable", err)
	}
	checksum = strings.ToLower(strings.TrimSpace(checksum))
	if info.Size != v.Size || checksum != v.ChecksumSHA256 || (info.ChecksumSHA256 != "" && info.ChecksumSHA256 != v.ChecksumSHA256) {
		return Metadata{}, apperror.Conflict("uploaded object does not match declared size or checksum", nil)
	}
	v.Status = "ready"
	if s.scanRequired {
		v.ScanStatus = "pending"
	} else {
		v.ScanStatus = "skipped"
	}
	v.UpdatedAt = s.now()
	v.UpdatedBy = caller.Subject
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if err := s.repository.Update(ctx, tx, v, expected); err != nil {
			return err
		}
		v.Version = expected + 1
		return s.addStatusEvent(ctx, tx, v)
	})
	v.Version = expected + 1
	return v, translate(err)
}
func (s *Service) Get(ctx context.Context, id, tenant string) (Metadata, error) {
	if _, ok := principal.FromContext(ctx); !ok {
		return Metadata{}, apperror.Unauthorized("authenticated actor is required")
	}
	v, err := s.repository.Get(ctx, id, tenant)
	return v, translate(err)
}
func (s *Service) AuthorizeDownload(ctx context.Context, id, tenant string) (Authorization, error) {
	if _, ok := principal.FromContext(ctx); !ok {
		return Authorization{}, apperror.Unauthorized("authenticated actor is required")
	}
	v, err := s.repository.Get(ctx, id, tenant)
	if err != nil {
		return Authorization{}, translate(err)
	}
	if v.Status != "ready" || (v.ScanStatus != "clean" && v.ScanStatus != "skipped") {
		return Authorization{}, apperror.Conflict("file is not available for download", nil)
	}
	urlValue, err := s.storage.PresignDownload(ctx, v.ObjectKey)
	if err != nil {
		return Authorization{}, apperror.Unavailable("object storage unavailable", err)
	}
	return Authorization{File: v, URL: urlValue.String(), ExpiresAt: s.now().Add(s.storage.TTL())}, nil
}
func (s *Service) ReportScanResult(ctx context.Context, id, tenant, scanStatus string, expected int64) (Metadata, error) {
	caller, ok := principal.FromContext(ctx)
	if !ok {
		return Metadata{}, apperror.Unauthorized("authenticated scanner is required")
	}
	if scanStatus != "clean" && scanStatus != "infected" {
		return Metadata{}, apperror.Invalid("scan_status must be clean or infected", nil)
	}
	v, err := s.repository.Get(ctx, id, tenant)
	if err != nil {
		return Metadata{}, translate(err)
	}
	if v.Status != "ready" || v.ScanStatus != "pending" {
		return Metadata{}, apperror.Conflict("file is not pending a scan result", nil)
	}
	v.ScanStatus, v.UpdatedAt, v.UpdatedBy = scanStatus, s.now(), caller.Subject
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if err := s.repository.Update(ctx, tx, v, expected); err != nil {
			return err
		}
		v.Version = expected + 1
		return s.addStatusEvent(ctx, tx, v)
	})
	v.Version = expected + 1
	return v, translate(err)
}
func (s *Service) Delete(ctx context.Context, id, tenant string, expected int64) (Metadata, error) {
	caller, ok := principal.FromContext(ctx)
	if !ok {
		return Metadata{}, apperror.Unauthorized("authenticated actor is required")
	}
	v, err := s.repository.Get(ctx, id, tenant)
	if err != nil {
		return Metadata{}, translate(err)
	}
	if err := s.storage.Delete(ctx, v.ObjectKey); err != nil {
		return Metadata{}, apperror.Unavailable("object storage unavailable", err)
	}
	v.Status = "deleted"
	v.UpdatedAt = s.now()
	v.UpdatedBy = caller.Subject
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if err := s.repository.Update(ctx, tx, v, expected); err != nil {
			return err
		}
		v.Version = expected + 1
		return s.addStatusEvent(ctx, tx, v)
	})
	v.Version = expected + 1
	return v, translate(err)
}
func (s *Service) addStatusEvent(ctx context.Context, tx sqlx.ExtContext, value Metadata) error {
	payload := &filev1.FileStatusChangedEvent{File: &filev1.FileMetadata{Id: value.ID, TenantId: value.TenantID, OwnerId: value.OwnerID, Bucket: value.Bucket, ObjectKey: value.ObjectKey, Filename: value.Filename, ContentType: value.ContentType, Size: value.Size, ChecksumSha256: value.ChecksumSHA256, Status: value.Status, ScanStatus: value.ScanStatus, Version: value.Version, CreatedAt: timestamppb.New(value.CreatedAt), UpdatedAt: timestamppb.New(value.UpdatedAt), CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy}}
	envelope, err := eventbus.NewEnvelope(eventbus.Metadata{EventID: uuid.NewString(), EventType: "platform.file.v1.FileStatusChanged", AggregateID: value.ID, AggregateType: "file", TenantID: value.TenantID, SchemaVersion: 1, ActorID: value.UpdatedBy, OccurredAt: value.UpdatedAt}, payload)
	if err != nil {
		return err
	}
	encoded, err := proto.Marshal(envelope)
	if err != nil {
		return err
	}
	return s.repository.AddOutbox(ctx, tx, OutboxEvent{ID: envelope.GetEventId(), Subject: "platform.file.status.changed.v1", Envelope: encoded, AvailableAt: value.UpdatedAt, CreatedAt: value.UpdatedAt, UpdatedAt: value.UpdatedAt, CreatedBy: value.UpdatedBy, UpdatedBy: value.UpdatedBy})
}
func validChecksum(v string) bool {
	decoded, err := hex.DecodeString(v)
	return err == nil && len(decoded) == 32
}
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("file not found")
	}
	if errors.Is(err, ErrStaleVersion) {
		return apperror.StaleVersion(err)
	}
	return apperror.Internal(err)
}

var Module = fx.Module("file", fx.Provide(objectstorage.New, NewRepository, NewRuntimeService))
