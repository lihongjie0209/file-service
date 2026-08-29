package httptransport

import (
	"log/slog"

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
	Filename       string `json:"filename" binding:"required"`
	ContentType    string `json:"content_type" binding:"required"`
	Size           int64  `json:"size" binding:"required"`
	ChecksumSHA256 string `json:"checksum_sha256" binding:"required"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}
type CompleteUploadRequest struct {
	ID              string `json:"id" binding:"required"`
	TenantID        string `json:"tenant_id" binding:"required"`
	ChecksumSHA256  string `json:"checksum_sha256" binding:"required"`
	ExpectedVersion int64  `json:"expected_version" binding:"required"`
}
type ReportScanResultRequest struct {
	ID              string `json:"id" binding:"required"`
	TenantID        string `json:"tenant_id" binding:"required"`
	ScanStatus      string `json:"scan_status" binding:"required"`
	ExpectedVersion int64  `json:"expected_version" binding:"required"`
}
type FileIDRequest struct {
	ID       string `json:"id" binding:"required"`
	TenantID string `json:"tenant_id" binding:"required"`
}
type DeleteFileRequest struct {
	ID              string `json:"id" binding:"required"`
	TenantID        string `json:"tenant_id" binding:"required"`
	ExpectedVersion int64  `json:"expected_version" binding:"required"`
}

type MeResponseBody struct {
	Subject string `json:"subject"`
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
// @Success 200 {object} Response{body=file.Authorization}
// @Router /api/v1/files/uploads/initiate [post]
func (h *Handler) InitiateUpload(c *gin.Context) {
	var r InitiateUploadRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.InitiateUpload(c.Request.Context(), r.TenantID, r.Filename, r.ContentType, r.Size, r.ChecksumSHA256, r.IdempotencyKey)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, v)
}

// CompleteUpload godoc
// @Summary Verify and complete an upload
// @Tags files
// @Security Bearer
// @Param request body CompleteUploadRequest true "Completed upload"
// @Success 200 {object} Response{body=file.Metadata}
// @Router /api/v1/files/uploads/complete [post]
func (h *Handler) CompleteUpload(c *gin.Context) {
	var r CompleteUploadRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.CompleteUpload(c.Request.Context(), r.ID, r.TenantID, r.ChecksumSHA256, r.ExpectedVersion)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, v)
}

// ReportScanResult godoc
// @Summary Record an antivirus or content scan result
// @Tags files
// @Accept json
// @Produce json
// @Security Bearer
// @Security PSK
// @Param request body ReportScanResultRequest true "Scan result"
// @Success 200 {object} Response{body=file.Metadata}
// @Router /api/v1/files/scans/report [post]
func (h *Handler) ReportScanResult(c *gin.Context) {
	var request ReportScanResultRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	value, err := h.files.ReportScanResult(c.Request.Context(), request.ID, request.TenantID, request.ScanStatus, request.ExpectedVersion)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, value)
}

// GetFile godoc
// @Summary Get file metadata
// @Tags files
// @Security Bearer
// @Param request body FileIDRequest true "File ID"
// @Success 200 {object} Response{body=file.Metadata}
// @Router /api/v1/files/metadata/get [post]
func (h *Handler) GetFile(c *gin.Context) {
	var r FileIDRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.Get(c.Request.Context(), r.ID, r.TenantID)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, v)
}

// AuthorizeDownload godoc
// @Summary Create a short-lived download URL
// @Tags files
// @Security Bearer
// @Param request body FileIDRequest true "File ID"
// @Success 200 {object} Response{body=file.Authorization}
// @Router /api/v1/files/downloads/authorize [post]
func (h *Handler) AuthorizeDownload(c *gin.Context) {
	var r FileIDRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.AuthorizeDownload(c.Request.Context(), r.ID, r.TenantID)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, v)
}

// DeleteFile godoc
// @Summary Delete a file using optimistic concurrency
// @Tags files
// @Security Bearer
// @Param request body DeleteFileRequest true "File version"
// @Success 200 {object} Response{body=file.Metadata}
// @Router /api/v1/files/delete [post]
func (h *Handler) DeleteFile(c *gin.Context) {
	var r DeleteFileRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid json request", err))
		return
	}
	v, err := h.files.Delete(c.Request.Context(), r.ID, r.TenantID, r.ExpectedVersion)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, v)
}

// CreateUser godoc
// @Summary Create a user
// @Tags users
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body CreateUserRequest true "User"
// @Success 200 {object} Response{body=user.User}
// @Failure 400 {object} Response "Code 10001: invalid request"
// @Failure 409 {object} Response "Code 30009: email already exists"

// GetUser godoc
// @Summary Get a user
// @Tags users
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body GetUserRequest true "User ID"
// @Success 200 {object} Response{body=user.User}
// @Failure 404 {object} Response "Code 10004: user not found"

// ListUsers godoc
// @Summary List users
// @Tags users
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body ListUsersRequest true "Pagination"
// @Success 200 {object} Response{body=user.Page}

// UpdateUser godoc
// @Summary Update a user using optimistic locking
// @Tags users
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body UpdateUserRequest true "User and current version"
// @Success 200 {object} Response{body=user.User}
// @Failure 409 {object} Response "Code 30009: version conflict"

// DeleteUser godoc
// @Summary Delete a user using optimistic locking
// @Tags users
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body DeleteUserRequest true "User ID and current version"
// @Success 200 {object} Response
