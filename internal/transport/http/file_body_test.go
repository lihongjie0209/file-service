package httptransport

import (
	"encoding/json"
	"strings"
	"testing"

	filedomain "github.com/lihongjie0209/file-service/internal/file"
)

func TestFileBodyOmitsStorageAndIdempotencyInternals(t *testing.T) {
	encoded, err := json.Marshal(fileBody(filedomain.Metadata{
		ID:                "file-1",
		Bucket:            "private-bucket",
		ObjectKey:         "tenant/private/file.csv",
		IdempotencyKey:    "request-secret",
		MultipartUploadID: "storage-upload-secret",
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"bucket", "object_key", "idempotency_key", "multipart_upload_id", "private-bucket", "tenant/private", "request-secret", "storage-upload-secret"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("public file body contains internal value %q: %s", secret, encoded)
		}
	}
	if !strings.Contains(string(encoded), `"id":"file-1"`) {
		t.Fatalf("public file body lost file identity: %s", encoded)
	}
}
