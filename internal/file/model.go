package file

import "time"

type Metadata struct {
	ID             string    `db:"id" json:"id"`
	TenantID       string    `db:"tenant_id" json:"tenant_id"`
	OwnerID        string    `db:"owner_id" json:"owner_id"`
	Bucket         string    `db:"bucket" json:"bucket"`
	ObjectKey      string    `db:"object_key" json:"object_key"`
	Filename       string    `db:"filename" json:"filename"`
	ContentType    string    `db:"content_type" json:"content_type"`
	Size           int64     `db:"size" json:"size"`
	ChecksumSHA256 string    `db:"checksum_sha256" json:"checksum_sha256"`
	Status         string    `db:"status" json:"status"`
	ScanStatus     string    `db:"scan_status" json:"scan_status"`
	IdempotencyKey string    `db:"idempotency_key" json:"idempotency_key"`
	Version        int64     `db:"version" json:"version"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
	CreatedBy      string    `db:"created_by" json:"created_by"`
	UpdatedBy      string    `db:"updated_by" json:"updated_by"`
}
type Authorization struct {
	File      Metadata          `json:"file"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt time.Time         `json:"expires_at"`
}
type OutboxEvent struct {
	ID, Subject                       string
	Envelope                          []byte
	AvailableAt, CreatedAt, UpdatedAt time.Time
	CreatedBy, UpdatedBy              string
}
