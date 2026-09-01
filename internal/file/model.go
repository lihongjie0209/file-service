package file

import "time"

type Metadata struct {
	ID                string     `db:"id" json:"id"`
	TenantID          string     `db:"tenant_id" json:"tenant_id"`
	ApplicationID     string     `db:"application_id" json:"application_id"`
	OwnerID           string     `db:"owner_id" json:"owner_id"`
	Bucket            string     `db:"bucket" json:"bucket"`
	ObjectKey         string     `db:"object_key" json:"object_key"`
	Filename          string     `db:"filename" json:"filename"`
	ContentType       string     `db:"content_type" json:"content_type"`
	Size              int64      `db:"size" json:"size"`
	ChecksumSHA256    string     `db:"checksum_sha256" json:"checksum_sha256"`
	Status            string     `db:"status" json:"status"`
	ScanStatus        string     `db:"scan_status" json:"scan_status"`
	IdempotencyKey    string     `db:"idempotency_key" json:"idempotency_key"`
	UploadMode        string     `db:"upload_mode" json:"upload_mode"`
	MultipartUploadID string     `db:"multipart_upload_id" json:"-"`
	PartSize          int64      `db:"part_size" json:"part_size"`
	PartCount         int32      `db:"part_count" json:"part_count"`
	UploadExpiresAt   *time.Time `db:"upload_expires_at" json:"upload_expires_at,omitempty"`
	Version           int64      `db:"version" json:"version"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
	CreatedBy         string     `db:"created_by" json:"created_by"`
	UpdatedBy         string     `db:"updated_by" json:"updated_by"`
}
type Authorization struct {
	File      Metadata          `json:"file"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt time.Time         `json:"expires_at"`
}
type MultipartAuthorization struct {
	File      Metadata  `json:"file"`
	UploadID  string    `json:"upload_id"`
	PartSize  int64     `json:"part_size"`
	PartCount int32     `json:"part_count"`
	ExpiresAt time.Time `json:"expires_at"`
}
type CompletedPart struct {
	PartNumber int32  `json:"part_number"`
	ETag       string `json:"etag"`
}
type ListFilter struct {
	TenantID      string
	ApplicationID string
	Keyword       string
	Status        string
	ScanStatus    string
	ContentType   string
	OwnerID       string
	Page          int
	PageSize      int
}
type MetadataPage struct {
	Files    []Metadata `json:"files"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}
type OutboxEvent struct {
	ID, Subject                       string
	Envelope                          []byte
	AvailableAt, CreatedAt, UpdatedAt time.Time
	CreatedBy, UpdatedBy              string
}
