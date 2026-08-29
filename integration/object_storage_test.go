//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/lihongjie0209/file-service/internal/config"
	"github.com/lihongjie0209/file-service/internal/objectstorage"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestS3PresignedLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{Started: true, ContainerRequest: testcontainers.ContainerRequest{Image: "minio/minio:RELEASE.2025-07-23T15-54-02Z", ExposedPorts: []string{"9000/tcp"}, Env: map[string]string{"MINIO_ROOT_USER": "minioadmin", "MINIO_ROOT_PASSWORD": "minioadmin"}, Cmd: []string{"server", "/data"}, WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(time.Minute)}})
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, container)
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := fmt.Sprintf("%s:%s", host, port.Port())
	admin, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4("minioadmin", "minioadmin", ""), Secure: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.MakeBucket(ctx, "platform-files", minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	storage, err := objectstorage.New(config.Config{ObjectStorage: config.ObjectStorage{Enabled: true, Endpoint: endpoint, AccessKey: "minioadmin", SecretKey: "minioadmin", Bucket: "platform-files", Region: "us-east-1", PresignTTL: time.Minute, MaxUploadBytes: 1 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("platform-object-storage")
	checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
	uploadURL, err := storage.PresignUpload(ctx, "tenant/file.txt", "text/plain", int64(len(payload)), checksum)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL.String(), bytes.NewReader(payload))
	request.Header.Set("Content-Type", "text/plain")
	request.Header.Set("X-Amz-Meta-Sha256", checksum)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("upload status=%d", response.StatusCode)
	}
	info, err := storage.Stat(ctx, "tenant/file.txt")
	if err != nil || info.Size != int64(len(payload)) || info.ChecksumSHA256 != checksum {
		t.Fatalf("info=%+v err=%v", info, err)
	}
	downloadURL, err := storage.PresignDownload(ctx, "tenant/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	download, err := http.Get(downloadURL.String()) //nolint:noctx // URL is short-lived and this integration context owns the container.
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(download.Body)
	_ = download.Body.Close()
	if !bytes.Equal(got, payload) {
		t.Fatalf("download=%q", got)
	}
	if err := storage.Delete(ctx, "tenant/file.txt"); err != nil {
		t.Fatal(err)
	}
	multipartPayload := bytes.Repeat([]byte("m"), 6<<20)
	multipartChecksum := fmt.Sprintf("%x", sha256.Sum256(multipartPayload))
	uploadID, err := storage.InitiateMultipart(ctx, "tenant/archive.bin", "application/octet-stream", multipartChecksum)
	if err != nil {
		t.Fatal(err)
	}
	parts := make([]objectstorage.CompletedPart, 0, 2)
	for index, body := range [][]byte{multipartPayload[:5<<20], multipartPayload[5<<20:]} {
		partNumber := index + 1
		partURL, err := storage.PresignUploadPart(ctx, "tenant/archive.bin", uploadID, partNumber)
		if err != nil {
			t.Fatal(err)
		}
		request, _ := http.NewRequestWithContext(ctx, http.MethodPut, partURL.String(), bytes.NewReader(body))
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("multipart part %d status=%d", partNumber, response.StatusCode)
		}
		parts = append(parts, objectstorage.CompletedPart{PartNumber: partNumber, ETag: response.Header.Get("ETag")})
	}
	if err := storage.CompleteMultipart(ctx, "tenant/archive.bin", uploadID, parts); err != nil {
		t.Fatal(err)
	}
	multipartInfo, err := storage.Stat(ctx, "tenant/archive.bin")
	if err != nil || multipartInfo.Size != int64(len(multipartPayload)) || multipartInfo.ChecksumSHA256 != multipartChecksum {
		t.Fatalf("multipart info=%+v err=%v", multipartInfo, err)
	}
}
