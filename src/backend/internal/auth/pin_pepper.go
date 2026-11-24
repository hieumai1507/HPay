package auth

// ADR-0018 stub: signatures only, so the red tests compile and fail for the right reason.

import (
	"context"
	"errors"

	"github.com/google/uuid"
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

// PepperSet holds the current pepper and the old ones that may still verify.
type PepperSet struct{}

// NewPepperSet validates and builds a PepperSet.
func NewPepperSet(current PepperKey, old ...PepperKey) (*PepperSet, error) {
	return nil, errors.New("not implemented")
}

// CurrentID is the key id new hashes are written with.
func (ps *PepperSet) CurrentID() string { return "" }

// HashPIN returns the encoded hash of a PIN under the current pepper.
func (ps *PepperSet) HashPIN(pin string) (string, error) { return "", errors.New("not implemented") }

// VerifyPINHash checks a PIN against a stored hash.
func (ps *PepperSet) VerifyPINHash(encoded, pin string) error { return errors.New("not implemented") }

// NeedsRehash reports whether a stored hash should be rewritten under the current key and parameters.
func (ps *PepperSet) NeedsRehash(encoded string) bool { return false }

// BurnWork spends one derivation with the current parameters (no timing oracle).
func (ps *PepperSet) BurnWork() {}

// pinMAC is HMAC-SHA256(pepper, pin).
func pinMAC(key []byte, pin string) []byte { return nil }

type argonHash struct {
	m, t, p uint32
	keyID   string
	hasK    bool
	salt    []byte
	sum     []byte
}

func parseArgonHash(encoded string) (argonHash, error) {
	return argonHash{}, errors.New("not implemented")
}

// SetPINPepper installs the pepper set used for every PIN hash.
func (s *Service) SetPINPepper(ps *PepperSet) { s.pins = ps }

// rehashPIN rewrites oldHash to the current format, only if the row still holds oldHash.
func (s *Service) rehashPIN(ctx context.Context, userID uuid.UUID, oldHash, pin string) (bool, error) {
	return false, errors.New("not implemented")
}

// beforeRehashWrite is a test hook called between computing the new hash and the guarded UPDATE.
var beforeRehashWrite func()

// deriveTap is a test hook that sees the parameters of every derivation.
var deriveTap func(t, m uint32, p uint8)
