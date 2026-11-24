package storage

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Cloudinary implements Store using Cloudinary's REST API.
type Cloudinary struct {
	cloudName string
	apiKey    string
	apiSecret string
	client    *http.Client
}

// NewCloudinary creates a new Cloudinary store.
func NewCloudinary(cloudName, apiKey, apiSecret string) *Cloudinary {
	return &Cloudinary{
		cloudName: cloudName,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// signature signs the upload parameters using SHA-1 hex-digest.
func (c *Cloudinary) signature(params map[string]string) string {
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}

	raw := strings.Join(parts, "&") + c.apiSecret
	h := sha1.New()
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil))
}

// Put uploads binary data to Cloudinary.
func (c *Cloudinary) Put(ctx context.Context, key string, data []byte) error {
	ext := filepath.Ext(key)
	publicID := strings.TrimSuffix(key, ext)
	// Cloudinary expects forward slashes for folder structure in public_id
	publicID = strings.ReplaceAll(publicID, "\\", "/")

	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	params := map[string]string{
		"public_id": publicID,
		"timestamp": timestamp,
	}

	sig := c.signature(params)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", key)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write data to form: %w", err)
	}

	_ = writer.WriteField("api_key", c.apiKey)
	_ = writer.WriteField("timestamp", timestamp)
	_ = writer.WriteField("public_id", publicID)
	_ = writer.WriteField("signature", sig)

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	resourceType := "image"
	if strings.ToLower(ext) == ".xlsx" {
		resourceType = "raw"
	}

	u := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/%s/upload", c.cloudName, resourceType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudinary upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Get downloads binary data from Cloudinary.
func (c *Cloudinary) Get(ctx context.Context, key string) ([]byte, error) {
	ext := filepath.Ext(key)
	resourceType := "image"
	if strings.ToLower(ext) == ".xlsx" {
		resourceType = "raw"
	}

	// Format: https://res.cloudinary.com/<cloud_name>/<resource_type>/upload/<key>
	cleanKey := strings.ReplaceAll(key, "\\", "/")
	u := fmt.Sprintf("https://res.cloudinary.com/%s/%s/upload/%s", c.cloudName, resourceType, cleanKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cloudinary download failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
