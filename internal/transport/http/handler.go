package httptransport

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lihongjie0209/file-service/internal/apperror"
	"github.com/lihongjie0209/file-service/internal/buildinfo"
	filedomain "github.com/lihongjie0209/file-service/internal/file"
	"github.com/lihongjie0209/file-service/internal/health"
)

type Handler struct {
	logger *slog.Logger
	health *health.Service

	files *filedomain.Service
}

func NewHandler(healthService *health.Service, fileService *filedomain.Service, logger *slog.Logger) *Handler {
	return &Handler{health: healthService, files: fileService, logger: logger}
}

type InitiateUploadRequest struct {
	TenantID       string `json:"tenant_id" binding:"required"`
	ApplicationID  string `json:"application_id" binding:"required"`
	Filename       string `json:"filename" binding:"required"`
	ContentType    string `json:"content_type" binding:"required"`
	Size           int64  `json:"size" binding:"required"`
	ChecksumSHA256 string `json:"checksum_sha256" binding:"required"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}
type CompleteUploadRequest struct {
	ID              string `json:"id" binding:"required"`
	TenantID        string `json:"tenant_id" binding:"required"`
	ApplicationID   string `json:"application_id" binding:"required"`
	ChecksumSHA256  string `json:"checksum_sha256" binding:"required"`
	ExpectedVersion int64  `json:"expected_version" binding:"required"`
}
type InitiateMultipartUploadRequest struct {
	InitiateUploadRequest
	PartSize int64 `json:"part_size"`
}
type AuthorizeUploadPartRequest struct {
	ID            string `json:"id" binding:"required"`
	TenantID      string `json:"tenant_id" binding:"required"`
	ApplicationID string `json:"application_id" binding:"required"`
	PartNumber    int32  `json:"part_number" binding:"required"`
}
type CompletedPartRequest struct {
	PartNumber int32  `json:"part_number" binding:"required"`
	ETag       string `json:"etag" binding:"required"`
}
type CompleteMultipartUploadRequest struct {
	ID              string                 `json:"id" binding:"required"`
	TenantID        string                 `json:"tenant_id" binding:"required"`
	ApplicationID   string                 `json:"application_id" binding:"required"`
	Parts           []CompletedPartRequest `json:"parts" binding:"required"`
	ChecksumSHA256  string                 `json:"checksum_sha256" binding:"required"`
	ExpectedVersion int64                  `json:"expected_version" binding:"required"`
}
type ReportScanResultRequest struct {
	ID              string `json:"id" binding:"required"`
	TenantID        string `json:"tenant_id" binding:"required"`
	ApplicationID   string `json:"application_id" binding:"required"`
	ScanStatus      string `json:"scan_status" binding:"required"`
	ExpectedVersion int64  `json:"expected_version" binding:"required"`
}
type FileIDRequest struct {
	ID            string `json:"id" binding:"required"`
	TenantID      string `json:"tenant_id" binding:"required"`
	ApplicationID string `json:"application_id" binding:"required"`
}
type ListFilesRequest struct {
	TenantID      string `json:"tenant_id" binding:"required"`
	ApplicationID string `json:"application_id" binding:"required"`
	Keyword       string `json:"keyword"`
	Status        string `json:"status"`
	ScanStatus    string `json:"scan_status"`
	ContentType   string `json:"content_type"`
	OwnerID       string `json:"owner_id"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}
type DeleteFileRequest struct {
	ID              string `json:"id" binding:"required"`
	TenantID        string `json:"tenant_id" binding:"required"`
	ApplicationID   string `json:"application_id" binding:"required"`
	ExpectedVersion int64  `json:"expected_version" binding:"required"`
}

type MeResponseBody struct {
	Subject string `json:"subject"`
}
type FileBody struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	ApplicationID   string     `json:"application_id"`
	OwnerID         string     `json:"owner_id"`
	Filename        string     `json:"filename"`
	ContentType     string     `json:"content_type"`
	Size            int64      `json:"size"`
	ChecksumSHA256  string     `json:"checksum_sha256"`
	Status          string     `json:"status"`
	ScanStatus      string     `json:"scan_status"`
	UploadMode      string     `json:"upload_mode"`
	PartSize        int64      `json:"part_size"`
	PartCount       int32      `json:"part_count"`
	UploadExpiresAt *time.Time `json:"upload_expires_at,omitempty"`
	Version         int64      `json:"version"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	CreatedBy       string     `json:"created_by"`
	UpdatedBy       string     `json:"updated_by"`
}
type FileAuthorizationBody struct {
	File      FileBody          `json:"file"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt time.Time         `json:"expires_at"`
}
type MultipartAuthorizationBody struct {
	File      FileBody  `json:"file"`
	PartSize  int64     `json:"part_size"`
	PartCount int32     `json:"part_count"`
	ExpiresAt time.Time `json:"expires_at"`
}
type FilePageBody struct {
	Files    []FileBody `json:"files"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

func fileBody(value filedomain.Metadata) FileBody {
	return FileBody{ID: value.ID, TenantID: value.TenantID, ApplicationID: value.ApplicationID, OwnerID: value.OwnerID, Filename: value.Filename, ContentType: value.ContentType, Size: value.Size, ChecksumSHA256: value.ChecksumSHA256, Status: value.Status, ScanStatus: value.ScanStatus, UploadMode: value.UploadMode, PartSize: value.PartSize, PartCount: value.PartCount, UploadExpiresAt: value.UploadExpiresAt, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy}
}
func authorizationBody(value filedomain.Authorization) FileAuthorizationBody {
	return FileAuthorizationBody{File: fileBody(value.File), URL: value.URL, Headers: value.Headers, ExpiresAt: value.ExpiresAt}
}
func multipartAuthorizationBody(value filedomain.MultipartAuthorization) MultipartAuthorizationBody {
	return MultipartAuthorizationBody{File: fileBody(value.File), PartSize: value.PartSize, PartCount: value.PartCount, ExpiresAt: value.ExpiresAt}
}
func filePageBody(value filedomain.MetadataPage) FilePageBody {
	files := make([]FileBody, len(value.Files))
	for i := range value.Files {
		files[i] = fileBody(value.Files[i])
	}
	return FilePageBody{Files: files, Total: value.Total, Page: value.Page, PageSize: value.PageSize}
}

// Login godoc
// @Summary Issue a JWT access token
// @Tags authentication
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Client credentials"
// @Success 200 {object} Response{body=LoginResponseBody}
// @Failure 400 {object} Response "Code 10001: invalid request"
// @Failure 401 {object} Response "Code 20001: invalid credentials"
// @Failure 429 {object} Response "Code 10029: rate limited"

// Live godoc
// @Summary Check process liveness
// @Tags operations
// @Produce json
// @Success 200 {object} Response{body=health.Status}
// @Router /live [post]
func (h *Handler) Live(c *gin.Context) { OK(c, h.health.Live()) }

// Ready godoc
// @Summary Check database and Redis readiness
// @Tags operations
// @Produce json
// @Success 200 {object} Response{body=health.Status}
// @Failure 503 {object} Response{body=health.Status} "Code 50003: dependency unavailable"
// @Router /ready [post]
func (h *Handler) Ready(c *gin.Context) {
	status, ready := h.health.Ready(c.Request.Context())
	if !ready {
		c.JSON(503, Response{Code: apperror.CodeDependencyUnavailable, Message: "service is not ready", Body: status, RequestID: requestID(c)})
		return
	}
	OK(c, status)
}

// Me godoc
// @Summary Return the authenticated subject
// @Tags authentication
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{body=MeResponseBody}
// @Failure 401 {object} Response "Code 20001: unauthorized"
// @Router /api/v1/me [post]
func (h *Handler) Me(c *gin.Context) {
	subject, _ := c.Get("subject")
	OK(c, gin.H{"subject": subject})
}

// Version godoc
// @Summary Return build and runtime version information
// @Tags operations
// @Produce json
// @Success 200 {object} Response{body=buildinfo.Info}
// @Router /api/v1/version [post]
func (h *Handler) Version(c *gin.Context) { OK(c, buildinfo.Current()) }

// @Summary Initiate a direct object-storage upload
// @Tags files
// @Security Bearer
// @Param request body InitiateUploadRequest true "Upload metadata"
// @Success 200 {object} Response{body=FileAuthorizationBody}
// @Router /api/v1/files/uploads/initiate [post]
func (h *Handler) InitiateUpload(c *gin.Context) {
	var r InitiateUploadRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.InitiateUpload(c.Request.Context(), r.TenantID, r.ApplicationID, r.Filename, r.ContentType, r.Size, r.ChecksumSHA256, r.IdempotencyKey)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, authorizationBody(v))
}

// InitiateMultipartUpload godoc
// @Summary Initiate a resumable multipart object-storage upload
// @Tags files
// @Security Bearer
// @Param request body InitiateMultipartUploadRequest true "Multipart upload metadata"
// @Success 200 {object} Response{body=MultipartAuthorizationBody}
// @Router /api/v1/files/uploads/multipart/initiate [post]
func (h *Handler) InitiateMultipartUpload(c *gin.Context) {
	var r InitiateMultipartUploadRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.InitiateMultipartUpload(c.Request.Context(), r.TenantID, r.ApplicationID, r.Filename, r.ContentType, r.Size, r.ChecksumSHA256, r.IdempotencyKey, r.PartSize)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, multipartAuthorizationBody(v))
}

// AuthorizeUploadPart godoc
// @Summary Create a short-lived URL for one multipart upload part
// @Tags files
// @Security Bearer
// @Param request body AuthorizeUploadPartRequest true "Upload part"
// @Success 200 {object} Response{body=FileAuthorizationBody}
// @Router /api/v1/files/uploads/multipart/authorize-part [post]
func (h *Handler) AuthorizeUploadPart(c *gin.Context) {
	var r AuthorizeUploadPartRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.AuthorizeUploadPart(c.Request.Context(), r.ID, r.TenantID, r.ApplicationID, r.PartNumber)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, authorizationBody(v))
}

// CompleteMultipartUpload godoc
// @Summary Assemble uploaded parts and verify the completed object
// @Tags files
// @Security Bearer
// @Param request body CompleteMultipartUploadRequest true "Completed multipart upload"
// @Success 200 {object} Response{body=FileBody}
// @Router /api/v1/files/uploads/multipart/complete [post]
func (h *Handler) CompleteMultipartUpload(c *gin.Context) {
	var r CompleteMultipartUploadRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	parts := make([]filedomain.CompletedPart, 0, len(r.Parts))
	for _, part := range r.Parts {
		parts = append(parts, filedomain.CompletedPart{PartNumber: part.PartNumber, ETag: part.ETag})
	}
	v, err := h.files.CompleteMultipartUpload(c.Request.Context(), r.ID, r.TenantID, r.ApplicationID, r.ChecksumSHA256, parts, r.ExpectedVersion)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, fileBody(v))
}

// AbortMultipartUpload godoc
// @Summary Abort a multipart upload and release its object-storage parts
// @Tags files
// @Security Bearer
// @Param request body DeleteFileRequest true "Multipart upload version"
// @Success 200 {object} Response{body=FileBody}
// @Router /api/v1/files/uploads/multipart/abort [post]
func (h *Handler) AbortMultipartUpload(c *gin.Context) {
	var r DeleteFileRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.AbortMultipartUpload(c.Request.Context(), r.ID, r.TenantID, r.ApplicationID, r.ExpectedVersion)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, fileBody(v))
}

// CompleteUpload godoc
// @Summary Verify and complete an upload
// @Tags files
// @Security Bearer
// @Param request body CompleteUploadRequest true "Completed upload"
// @Success 200 {object} Response{body=FileBody}
// @Router /api/v1/files/uploads/complete [post]
func (h *Handler) CompleteUpload(c *gin.Context) {
	var r CompleteUploadRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.CompleteUpload(c.Request.Context(), r.ID, r.TenantID, r.ApplicationID, r.ChecksumSHA256, r.ExpectedVersion)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, fileBody(v))
}

// ReportScanResult godoc
// @Summary Record an antivirus or content scan result
// @Tags files
// @Accept json
// @Produce json
// @Security Bearer
// @Security PSK
// @Param request body ReportScanResultRequest true "Scan result"
// @Success 200 {object} Response{body=FileBody}
// @Router /api/v1/files/scans/report [post]
func (h *Handler) ReportScanResult(c *gin.Context) {
	var request ReportScanResultRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	value, err := h.files.ReportScanResult(c.Request.Context(), request.ID, request.TenantID, request.ApplicationID, request.ScanStatus, request.ExpectedVersion)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, fileBody(value))
}

// GetFile godoc
// @Summary Get file metadata
// @Tags files
// @Security Bearer
// @Param request body FileIDRequest true "File ID"
// @Success 200 {object} Response{body=FileBody}
// @Router /api/v1/files/metadata/get [post]
func (h *Handler) GetFile(c *gin.Context) {
	var r FileIDRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.Get(c.Request.Context(), r.ID, r.TenantID, r.ApplicationID)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, fileBody(v))
}

// ListFiles godoc
// @Summary List tenant file metadata
// @Tags files
// @Security Bearer
// @Param request body ListFilesRequest true "File filters and pagination"
// @Success 200 {object} Response{body=FilePageBody}
// @Failure 403 {object} Response "Code 20003: tenant access denied"
// @Router /api/v1/files/metadata/list [post]
func (h *Handler) ListFiles(c *gin.Context) {
	var request ListFilesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	page, err := h.files.List(c.Request.Context(), filedomain.ListFilter{
		TenantID: request.TenantID, ApplicationID: request.ApplicationID, Keyword: request.Keyword, Status: request.Status,
		ScanStatus: request.ScanStatus, ContentType: request.ContentType, OwnerID: request.OwnerID,
		Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, filePageBody(page))
}

// AuthorizeDownload godoc
// @Summary Create a short-lived download URL
// @Tags files
// @Security Bearer
// @Param request body FileIDRequest true "File ID"
// @Success 200 {object} Response{body=FileAuthorizationBody}
// @Router /api/v1/files/downloads/authorize [post]
func (h *Handler) AuthorizeDownload(c *gin.Context) {
	var r FileIDRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.AuthorizeDownload(c.Request.Context(), r.ID, r.TenantID, r.ApplicationID)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, authorizationBody(v))
}

// DeleteFile godoc
// @Summary Delete a file using optimistic concurrency
// @Tags files
// @Security Bearer
// @Param request body DeleteFileRequest true "File version"
// @Success 200 {object} Response{body=FileBody}
// @Router /api/v1/files/delete [post]
func (h *Handler) DeleteFile(c *gin.Context) {
	var r DeleteFileRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.Delete(c.Request.Context(), r.ID, r.TenantID, r.ApplicationID, r.ExpectedVersion)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, fileBody(v))
}
