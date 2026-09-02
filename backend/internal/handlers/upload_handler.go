package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"nowdone/internal/service"
)

// UploadHandler accepts multipart file uploads for task/note attachments and
// returns their S3 URL, per the rich-description attachment spec.
type UploadHandler struct {
	s3  *service.S3Service
	log *slog.Logger
}

func NewUploadHandler(s3 *service.S3Service, log *slog.Logger) *UploadHandler {
	return &UploadHandler{s3: s3, log: log}
}

const maxUploadSize = 25 << 20 // 25 MB — applies only to the legacy Upload path

// presignRequest is the body of POST /uploads/presign.
type presignRequest struct {
	Filename    string `json:"filename" binding:"required"`
	ContentType string `json:"contentType"`
}

// POST /uploads/presign
//
// Returns a short-lived presigned S3 PUT URL so the browser uploads the file
// straight to object storage — the API never sees the bytes. The client then
// saves the returned fileUrl into the task/note "attachments" list exactly as
// with the legacy flow. File size is bounded only by S3's own limits.
func (h *UploadHandler) Presign(c *gin.Context) {
	var req presignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "укажите имя файла")
		return
	}

	res, err := h.s3.PresignPut(c.Request.Context(), req.Filename, req.ContentType)
	if err != nil {
		h.log.Error("presign upload", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось подготовить загрузку файла")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"uploadUrl":   res.UploadURL,
		"fileUrl":     res.FileURL,
		"key":         res.Key, // let the client track it for orphan cleanup
		"contentType": res.ContentType,
		"expiresIn":   int(res.ExpiresIn.Seconds()),
	})
}

// DELETE /uploads?keys=attachments/a.jpg,attachments/b.png
//
// Best-effort removal of objects uploaded via a presigned URL that never got
// attached to a saved task/note — e.g. the user filled the "new task" dialog,
// uploaded files, then cancelled. Only keys under the attachments/ prefix are
// accepted; unknown keys are ignored. Never errors on a missing object.
func (h *UploadHandler) DeleteOrphans(c *gin.Context) {
	raw := c.Query("keys")
	if raw == "" {
		c.JSON(http.StatusOK, gin.H{"deleted": 0})
		return
	}

	deleted := 0
	for _, key := range strings.Split(raw, ",") {
		key = strings.TrimSpace(key)
		if key == "" || !strings.HasPrefix(key, "attachments/") {
			continue
		}
		if err := h.s3.DeleteObject(c.Request.Context(), key); err != nil {
			h.log.Warn("delete orphan attachment", "key", key, "error", err)
			continue
		}
		deleted++
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

// POST /uploads (multipart form field "file")
//
// Legacy path: the client streams the file to the API, which forwards it to S3.
// Kept as a fallback for when the browser cannot PUT to S3 directly (e.g. bucket
// CORS not configured yet). Capped at maxUploadSize.
func (h *UploadHandler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		respondError(c, http.StatusBadRequest, "файл не найден в запросе")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		h.log.Error("open uploaded file", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось прочитать файл")
		return
	}
	defer file.Close()

	url, err := h.s3.Upload(c.Request.Context(), fileHeader.Filename, file)
	if err != nil {
		h.log.Error("upload to s3", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось загрузить файл")
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url, "name": fileHeader.Filename})
}
