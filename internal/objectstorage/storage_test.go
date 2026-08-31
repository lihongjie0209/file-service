package objectstorage

import (
	"testing"
	"time"

	"github.com/lihongjie0209/file-service/internal/config"
)

func TestPresignUsesPublicEndpointWithoutChangingInternalClient(t *testing.T) {
	storage, err := New(config.Config{ObjectStorage: config.ObjectStorage{
		Enabled: true, Endpoint: "minio:9000", PresignEndpoint: "127.0.0.1:9000",
		AccessKey: "access", SecretKey: "secret", Bucket: "files", Region: "us-east-1",
		PresignTTL: 15 * time.Minute, MaxUploadBytes: 1024,
	}})
	if err != nil {
		t.Fatal(err)
	}
	s3 := storage.(*S3)
	if got := s3.client.EndpointURL().Host; got != "minio:9000" {
		t.Fatalf("internal endpoint = %q", got)
	}
	value, err := s3.PresignDownload(t.Context(), "tenant/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if value.Host != "127.0.0.1:9000" {
		t.Fatalf("presigned host = %q", value.Host)
	}
}
