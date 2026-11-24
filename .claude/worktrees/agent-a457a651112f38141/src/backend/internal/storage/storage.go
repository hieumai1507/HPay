// Package storage abstracts KYC image persistence (docs 01 §8: local dir for
// the portfolio, interface ready for a MinIO/S3 driver) and issues short-lived
// HMAC-signed URLs so the admin SPA can render images without an auth header
// (docs 01 §3.4: 5-minute signed links).
package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotFound         = errors.New("stored object not found")
	ErrInvalidSignature = errors.New("invalid file signature")
	ErrLinkExpired      = errors.New("signed link expired")
	ErrBadKey           = errors.New("invalid storage key")
)

// Store persists and retrieves immutable blobs by key.
type Store interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// --- local driver -------------------------------------------------------------

type Local struct {
	root string
}

func NewLocal(root string) (*Local, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}
	return &Local{root: root}, nil
}

// safePath rejects traversal — keys are internal ({request_id}/{name}.jpg)
// but the signed-URL route makes them caller-supplied, so validate hard.
func (l *Local) safePath(key string) (string, error) {
	if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") || strings.HasPrefix(key, "\\") {
		return "", ErrBadKey
	}
	clean := filepath.Clean(filepath.Join(l.root, filepath.FromSlash(key)))
	rootAbs, err := filepath.Abs(l.root)
	if err != nil {
		return "", err
	}
	cleanAbs, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(cleanAbs, rootAbs+string(filepath.Separator)) {
		return "", ErrBadKey
	}
	return clean, nil
}

func (l *Local) Put(_ context.Context, key string, data []byte) error {
	path, err := l.safePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}

func (l *Local) Get(_ context.Context, key string) ([]byte, error) {
	path, err := l.safePath(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return data, nil
}

// --- signed URLs ---------------------------------------------------------------

// Signer mints and verifies HMAC-signed, expiring links for stored objects.
// The signature IS the authorization — no JWT needed on the file route.
type Signer struct {
	secret []byte
}

func NewSigner(secret string) *Signer { return &Signer{secret: []byte(secret)} }

func (s *Signer) SignMsg(msg string, exp int64) string {
	m := hmac.New(sha256.New, s.secret)
	fmt.Fprintf(m, "%s\n%d", msg, exp)
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (s *Signer) VerifyMsg(msg string, exp int64, sig string) bool {
	want := s.SignMsg(msg, exp)
	return hmac.Equal([]byte(want), []byte(sig))
}

func (s *Signer) mac(key string, exp int64) string {
	return s.SignMsg(key, exp)
}

// SignURL returns the path+query for GET /v1/files/{key}?exp=&sig=. The key is
// the full storage key (e.g. "{uuid}/id_card.jpg" for KYC, "recon/{date}.xlsx"
// for reconciliation reports) so one route serves every stored artifact.
func (s *Signer) SignURL(key string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	return fmt.Sprintf("/v1/files/%s?exp=%d&sig=%s", key, exp, s.mac(key, exp))
}

// Verify checks signature first (never leak expiry info on forged links),
// then expiry.
func (s *Signer) Verify(key, expStr, sig string) error {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return ErrInvalidSignature
	}
	want := s.mac(key, exp)
	if !hmac.Equal([]byte(want), []byte(sig)) {
		return ErrInvalidSignature
	}
	if time.Now().Unix() > exp {
		return ErrLinkExpired
	}
	return nil
}
