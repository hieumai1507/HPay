package auth

// Peppered PIN hash (ADR-0018, spec w5-pin-pepper).
//
//	hash = argon2id(HMAC-SHA256(pepper, pin), salt, m=19456 KiB, t=2, p=1)
//	form = $argon2id$v=19$m=19456,t=2,p=1,k=<pepper id>$<salt>$<hash>
//
// The pepper lives only in configuration (PIN_PEPPER), never in the database. A hash without k= is a
// legacy hash: raw PIN, its own parameters (login-grade m=65536, t=3, p=2). Both verify side by side
// and the legacy ones are rewritten after the next successful verification (pin.go).
//
// Nothing here logs. Errors carry a key id at most, never a PIN, a pepper or an HMAC output.

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/argon2"

	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// Current PIN hash parameters (OWASP minimum for argon2id).
const (
	pinArgonTime    = 2
	pinArgonMemory  = 19456 // KiB
	pinArgonThreads = 1
	pinArgonKeyLen  = 32
	pinArgonSaltLen = 16
)

var (
	// ErrPepperNotConfigured: the service has no PIN pepper; PIN hashing fails closed.
	ErrPepperNotConfigured = errors.New("PIN pepper is not configured")
	// ErrUnknownPepperID: a stored hash names a key id that is not configured (rotated away).
	ErrUnknownPepperID = errors.New("PIN hash uses a pepper key id that is not configured")
	// ErrPINHashMismatch: the PIN does not match the stored hash.
	ErrPINHashMismatch = errors.New("PIN hash mismatch")
)

// PepperKey is one pepper: an id (stored inside the hash) and 32 secret bytes.
type PepperKey struct {
	ID  string
	Key []byte
}

// PepperSet holds the current pepper and the old ones that may still verify. It is immutable once
// built and safe for concurrent use.
type PepperSet struct {
	currentID string
	keys      map[string][]byte
}

// NewPepperSet validates and builds a PepperSet: ids match [a-z0-9-]{1,16} and are unique, every
// key is 32 bytes. The error never contains key bytes.
func NewPepperSet(current PepperKey, old ...PepperKey) (*PepperSet, error) {
	ps := &PepperSet{currentID: current.ID, keys: make(map[string][]byte, 1+len(old))}
	for _, k := range append([]PepperKey{current}, old...) {
		if !pepperIDPattern.MatchString(k.ID) {
			return nil, errors.New("pepper key id must match [a-z0-9-]{1,16}")
		}
		if len(k.Key) != 32 {
			return nil, fmt.Errorf("pepper %q must be 32 bytes, got %d", k.ID, len(k.Key))
		}
		if _, dup := ps.keys[k.ID]; dup {
			return nil, fmt.Errorf("pepper key id %q is used twice", k.ID)
		}
		ps.keys[k.ID] = append([]byte(nil), k.Key...)
	}
	return ps, nil
}

// PepperSetFromConfig builds the PepperSet from the loaded configuration (ADR-0018).
func PepperSetFromConfig(cfg config.Config) (*PepperSet, error) {
	old := make([]PepperKey, 0, len(cfg.PINPepperOld))
	for _, k := range cfg.PINPepperOld {
		old = append(old, PepperKey{ID: k.ID, Key: k.Key})
	}
	return NewPepperSet(PepperKey{ID: cfg.PINPepperID, Key: cfg.PINPepper}, old...)
}

// CurrentID is the key id new hashes are written with.
func (ps *PepperSet) CurrentID() string {
	if ps == nil {
		return ""
	}
	return ps.currentID
}

// pinMAC is HMAC-SHA256(pepper, pin): the 32 bytes that replace the PIN as the argon2 password.
func pinMAC(key []byte, pin string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(pin))
	return m.Sum(nil)
}

// HashPIN returns the encoded hash of a PIN under the current pepper.
func (ps *PepperSet) HashPIN(pin string) (string, error) {
	if ps == nil || len(ps.keys[ps.currentID]) != 32 {
		return "", ErrPepperNotConfigured
	}
	salt := make([]byte, pinArgonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := deriveKey(pinMAC(ps.keys[ps.currentID], pin), salt, pinArgonTime, pinArgonMemory, pinArgonThreads, pinArgonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d,k=%s$%s$%s",
		argon2.Version, pinArgonMemory, pinArgonTime, pinArgonThreads, ps.currentID,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(sum)), nil
}

// BurnWork spends one derivation with the current parameters, so an unknown user, a user without a
// PIN or an unusable hash cost the same as a real check (no timing oracle).
func (ps *PepperSet) BurnWork() {
	var salt [pinArgonSaltLen]byte // a fixed salt is fine: the result is discarded
	_ = deriveKey([]byte("burn"), salt[:], pinArgonTime, pinArgonMemory, pinArgonThreads, pinArgonKeyLen)
}

// VerifyPINHash checks a PIN against a stored hash in constant time. The result is ErrPINHashMismatch
// for a wrong PIN, ErrUnknownPepperID (wrapped, with the key id) when the key was rotated away,
// errMalformedHash for an unusable value, and ErrPepperNotConfigured when there is no pepper. An
// unusable hash still spends one derivation with the current parameters.
func (ps *PepperSet) VerifyPINHash(encoded, pin string) error {
	if ps == nil || len(ps.keys[ps.currentID]) != 32 {
		return ErrPepperNotConfigured
	}
	h, err := parseArgonHash(encoded)
	if err != nil {
		ps.BurnWork()
		return err
	}
	input := []byte(pin) // legacy: the raw PIN, as it was hashed before ADR-0018
	if h.hasK {
		key, ok := ps.keys[h.keyID]
		if !ok {
			ps.BurnWork()
			return fmt.Errorf("%w: %s", ErrUnknownPepperID, h.keyID)
		}
		input = pinMAC(key, pin)
	}
	got := deriveKey(input, h.salt, h.t, h.m, uint8(h.p), uint32(len(h.sum)))
	if subtle.ConstantTimeCompare(got, h.sum) != 1 {
		return ErrPINHashMismatch
	}
	return nil
}

// NeedsRehash reports whether a stored hash should be rewritten under the current key and
// parameters: legacy, another key id, other parameters or unreadable.
func (ps *PepperSet) NeedsRehash(encoded string) bool {
	h, err := parseArgonHash(encoded)
	if err != nil || !h.hasK || ps == nil {
		return true
	}
	return h.keyID != ps.currentID || h.m != pinArgonMemory || h.t != pinArgonTime ||
		h.p != pinArgonThreads || len(h.sum) != pinArgonKeyLen
}

// SetPINPepper installs the pepper set used for every PIN hash. Without one, PIN hashing and
// verification fail closed.
func (s *Service) SetPINPepper(ps *PepperSet) { s.pins = ps }

// beforeRehashWrite is a test hook called between computing the new hash and the guarded UPDATE.
var beforeRehashWrite func()

// rehashPIN rewrites oldHash to the current format. The UPDATE only matches while the row still
// holds oldHash, so a PIN change or reset that landed after the verification is never overwritten.
// It reports whether this call wrote the row.
func (s *Service) rehashPIN(ctx context.Context, userID uuid.UUID, oldHash, pin string) (bool, error) {
	fresh, err := s.pins.HashPIN(pin)
	if err != nil {
		return false, err
	}
	if beforeRehashWrite != nil {
		beforeRehashWrite()
	}
	n, err := s.q.RehashUserPIN(ctx, store.RehashUserPINParams{
		ID:      userID,
		OldHash: pgtype.Text{String: oldHash, Valid: true},
		NewHash: pgtype.Text{String: fresh, Valid: true},
	})
	return n == 1, err
}
