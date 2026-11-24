package storage

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockTransport struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestCloudinarySignature(t *testing.T) {
	c := NewCloudinary("my-cloud", "my-key", "my-secret")
	params := map[string]string{
		"public_id": "test_public_id",
		"timestamp": "1234567890",
	}

	sig := c.signature(params)
	// Expected signature computation:
	// sorted query string: "public_id=test_public_id&timestamp=1234567890" + "my-secret"
	// -> SHA-1 hash of "public_id=test_public_id&timestamp=1234567890my-secret"
	// Let's compute manually or check if matches
	expected := "d2e3c0884ea33102afb4998cf9de0a4d6b3f0c02"
	if sig != expected {
		t.Errorf("expected signature %s, got %s", expected, sig)
	}
}

func TestCloudinaryPutAndGet(t *testing.T) {
	ctx := context.Background()

	var lastURL string
	var lastMethod string
	var lastContentType string

	transport := &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			lastURL = req.URL.String()
			lastMethod = req.Method
			lastContentType = req.Header.Get("Content-Type")

			if req.Method == http.MethodPost {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"public_id":"test-key"}`)),
				}, nil
			}

			if req.Method == http.MethodGet {
				if strings.Contains(req.URL.Path, "missing") {
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("file-content")),
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader("bad request")),
			}, nil
		},
	}

	c := NewCloudinary("my-cloud", "my-key", "my-secret")
	c.client.Transport = transport

	// Test 1: Put image
	err := c.Put(ctx, "kyc/selfie.jpg", []byte("selfie-bytes"))
	if err != nil {
		t.Fatalf("Put image failed: %v", err)
	}
	if lastMethod != http.MethodPost {
		t.Errorf("expected POST upload, got %s", lastMethod)
	}
	if !strings.Contains(lastURL, "/my-cloud/image/upload") {
		t.Errorf("unexpected upload URL: %s", lastURL)
	}
	if !strings.Contains(lastContentType, "multipart/form-data") {
		t.Errorf("expected multipart Content-Type, got %s", lastContentType)
	}

	// Test 2: Put raw file (.xlsx)
	err = c.Put(ctx, "recon/report.xlsx", []byte("excel-bytes"))
	if err != nil {
		t.Fatalf("Put raw failed: %v", err)
	}
	if !strings.Contains(lastURL, "/my-cloud/raw/upload") {
		t.Errorf("expected raw upload URL for .xlsx, got %s", lastURL)
	}

	// Test 3: Get image
	data, err := c.Get(ctx, "kyc/selfie.jpg")
	if err != nil {
		t.Fatalf("Get image failed: %v", err)
	}
	if string(data) != "file-content" {
		t.Errorf("expected 'file-content', got %q", string(data))
	}
	if lastURL != "https://res.cloudinary.com/my-cloud/image/upload/kyc/selfie.jpg" {
		t.Errorf("unexpected download URL: %s", lastURL)
	}

	// Test 4: Get missing image
	_, err = c.Get(ctx, "kyc/missing.jpg")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for missing file, got %v", err)
	}
}
