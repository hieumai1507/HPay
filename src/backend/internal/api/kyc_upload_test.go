package api

// KYC upload and file serving at the HTTP layer (docs/specs/w3-kyc-upload).
// Cases: AC-6, AC-8. No database needed: handlers are exercised through helpers.

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// AC-6: the whole body is capped BEFORE the multipart parser runs.
func TestKYCSubmitBodyIsCappedBeforeParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reached := false
	r := gin.New()
	r.POST("/v1/kyc/submit", limitKYCBody(), func(c *gin.Context) {
		// Stand-in for the real handler: it reads the form the way readImagePart does.
		if _, err := c.FormFile("id_card"); err != nil {
			respondTooLarge(c, err)
			return
		}
		reached = true
		c.Status(http.StatusAccepted)
	})

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("id_card", "x.jpg")
	_, _ = part.Write(bytes.Repeat([]byte{0xAB}, 13<<20)) // 13 MiB, above the 12 MiB request cap
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/kyc/submit", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("a 13 MiB body must be refused with 413, got %d: %s", w.Code, w.Body.String())
	}
	if reached {
		t.Fatal("the handler ran on an oversized body")
	}
}

func TestKYCSubmitNormalBodyPasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/kyc/submit", limitKYCBody(), func(c *gin.Context) {
		if _, err := c.FormFile("id_card"); err != nil {
			respondTooLarge(c, err)
			return
		}
		c.Status(http.StatusAccepted)
	})
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("id_card", "x.jpg")
	_, _ = part.Write(bytes.Repeat([]byte{0xAB}, 2<<20))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/kyc/submit", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("a 2 MiB upload must pass, got %d", w.Code)
	}
}

// AC-8: how stored files are served.
func TestServeFileHeaders(t *testing.T) {
	cases := []struct {
		key             string
		wantType        string
		wantDisposition string
	}{
		{"req/id_card.jpg", "image/jpeg", "inline"},
		{"req/selfie.png", "image/png", "inline"},
		{"recon/2026-01-01.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "attachment"},
		{"req/evil.html", "application/octet-stream", "attachment"},
		{"req/noext", "application/octet-stream", "attachment"},
		{"req/archive.JPG", "image/jpeg", "inline"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			writeStoredFile(c, tc.key, []byte("<html><script>alert(1)</script></html>"))
			h := w.Header()
			if got := h.Get("Content-Type"); got != tc.wantType {
				t.Errorf("Content-Type = %q, want %q (it must come from the key extension, never from sniffing the bytes)", got, tc.wantType)
			}
			if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if got := h.Get("Content-Disposition"); !strings.HasPrefix(got, tc.wantDisposition) {
				t.Errorf("Content-Disposition = %q, want it to start with %q", got, tc.wantDisposition)
			}
			if got := h.Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q, want private, no-store", got)
			}
		})
	}
}

func TestServeFileFilenameIsSanitized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeStoredFile(c, "req/a\"b\r\nX-Injected: 1.xlsx", []byte("x"))
	cd := w.Header().Get("Content-Disposition")
	if strings.ContainsAny(cd, "\r\n") || strings.Contains(w.Header().Get("X-Injected"), "1") {
		t.Fatalf("header injection through the file name: %q", cd)
	}
}
