package api

// HTTP-level guards for KYC uploads and for serving stored files
// (docs/specs/w3-kyc-upload, audit BE-H9).

import (
	"errors"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// kycMaxRequestBytes caps the WHOLE multipart request: two images of at most 5 MiB
// each plus form overhead. It is applied before the multipart parser runs, so a very
// large body or many parts cannot be buffered first and rejected later.
const kycMaxRequestBytes = 12 << 20

// limitKYCBody wraps the request body in http.MaxBytesReader.
func limitKYCBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, kycMaxRequestBytes)
		c.Next()
	}
}

// respondTooLarge turns a body-limit error into 413 and anything else into a
// validation error.
func respondTooLarge(c *gin.Context, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
			"code": "ERR_PAYLOAD_TOO_LARGE", "message": "the upload is too large",
			"request_id": c.GetString("request_id"),
		})
		return
	}
	respondValidation(c, "multipart field is required")
}

// storedFileType maps a storage key to a content type by its EXTENSION only. The
// bytes are never sniffed: a hostile file renamed to look like an image must not be
// served as HTML.
func storedFileType(key string) (contentType string, inline bool) {
	switch strings.ToLower(path.Ext(key)) {
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".png":
		return "image/png", true
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", false
	default:
		return "application/octet-stream", false
	}
}

// writeStoredFile serves a stored artifact with headers that stop a browser from
// sniffing, rendering or caching it: nosniff, a type derived from the key, a
// sanitized Content-Disposition, and private no-store caching.
func writeStoredFile(c *gin.Context, key string, data []byte) {
	ct, inline := storedFileType(key)
	disp := "attachment"
	if inline {
		disp = "inline"
	}
	name := sanitizeFilename(path.Base(key))
	h := c.Writer.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, no-store")
	h.Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": name}))
	c.Data(http.StatusOK, ct, data)
}

// sanitizeFilename keeps a conservative character set so a key can never inject
// header content or path separators.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "file"
	}
	return out
}
