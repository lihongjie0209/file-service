package objectstorage

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lihongjie0209/file-service/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectInfo struct {
	Size           int64
	ChecksumSHA256 string
}
type CompletedPart struct {
	PartNumber int
	ETag       string
}
type Storage interface {
	PresignUpload(context.Context, string, string, int64, string) (*url.URL, error)
	PresignDownload(context.Context, string) (*url.URL, error)
	Stat(context.Context, string) (ObjectInfo, error)
	Delete(context.Context, string) error
	InitiateMultipart(context.Context, string, string, string) (string, error)
	PresignUploadPart(context.Context, string, string, int) (*url.URL, error)
	CompleteMultipart(context.Context, string, string, []CompletedPart) error
	AbortMultipart(context.Context, string, string) error
	Bucket() string
	TTL() time.Duration
	MaxUploadBytes() int64
	Enabled() bool
}
type S3 struct {
	client *minio.Client
	cfg    config.ObjectStorage
}

func New(cfg config.Config) (Storage, error) {
	c := cfg.ObjectStorage
	if !c.Enabled {
		return &S3{cfg: c}, nil
	}
	client, err := minio.New(c.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(c.AccessKey, c.SecretKey, c.SessionToken), Secure: c.UseSSL, Region: c.Region})
	if err != nil {
		return nil, err
	}
	return &S3{client: client, cfg: c}, nil
}
func (s *S3) Enabled() bool         { return s != nil && s.cfg.Enabled }
func (s *S3) Bucket() string        { return s.cfg.Bucket }
func (s *S3) TTL() time.Duration    { return s.cfg.PresignTTL }
func (s *S3) MaxUploadBytes() int64 { return s.cfg.MaxUploadBytes }
func (s *S3) PresignUpload(ctx context.Context, key, contentType string, size int64, checksum string) (*url.URL, error) {
	if !s.Enabled() {
		return nil, errors.New("object storage is disabled")
	}
	headers := make(http.Header)
	headers.Set("Content-Type", contentType)
	headers.Set("Content-Length", strconv.FormatInt(size, 10))
	headers.Set("X-Amz-Meta-Sha256", checksum)
	return s.client.PresignHeader(ctx, http.MethodPut, s.cfg.Bucket, key, s.cfg.PresignTTL, nil, headers)
}
func (s *S3) PresignDownload(ctx context.Context, key string) (*url.URL, error) {
	if !s.Enabled() {
		return nil, errors.New("object storage is disabled")
	}
	return s.client.PresignedGetObject(ctx, s.cfg.Bucket, key, s.cfg.PresignTTL, nil)
}
func (s *S3) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	if !s.Enabled() {
		return ObjectInfo{}, errors.New("object storage is disabled")
	}
	info, err := s.client.StatObject(ctx, s.cfg.Bucket, key, minio.StatObjectOptions{})
	return ObjectInfo{Size: info.Size, ChecksumSHA256: info.Metadata.Get("X-Amz-Meta-Sha256")}, err
}
func (s *S3) Delete(ctx context.Context, key string) error {
	if !s.Enabled() {
		return errors.New("object storage is disabled")
	}
	return s.client.RemoveObject(ctx, s.cfg.Bucket, key, minio.RemoveObjectOptions{})
}
func (s *S3) InitiateMultipart(ctx context.Context, key, contentType, checksum string) (string, error) {
	if !s.Enabled() {
		return "", errors.New("object storage is disabled")
	}
	core := minio.Core{Client: s.client}
	return core.NewMultipartUpload(ctx, s.cfg.Bucket, key, minio.PutObjectOptions{ContentType: contentType, UserMetadata: map[string]string{"sha256": checksum}})
}
func (s *S3) PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int) (*url.URL, error) {
	if !s.Enabled() {
		return nil, errors.New("object storage is disabled")
	}
	params := url.Values{"uploadId": {uploadID}, "partNumber": {strconv.Itoa(partNumber)}}
	return s.client.Presign(ctx, http.MethodPut, s.cfg.Bucket, key, s.cfg.PresignTTL, params)
}
func (s *S3) CompleteMultipart(ctx context.Context, key, uploadID string, parts []CompletedPart) error {
	if !s.Enabled() {
		return errors.New("object storage is disabled")
	}
	completed := make([]minio.CompletePart, 0, len(parts))
	for _, part := range parts {
		completed = append(completed, minio.CompletePart{PartNumber: part.PartNumber, ETag: strings.Trim(strings.TrimSpace(part.ETag), "\"")})
	}
	core := minio.Core{Client: s.client}
	_, err := core.CompleteMultipartUpload(ctx, s.cfg.Bucket, key, uploadID, completed, minio.PutObjectOptions{})
	return err
}
func (s *S3) AbortMultipart(ctx context.Context, key, uploadID string) error {
	if !s.Enabled() {
		return errors.New("object storage is disabled")
	}
	core := minio.Core{Client: s.client}
	return core.AbortMultipartUpload(ctx, s.cfg.Bucket, key, uploadID)
}
