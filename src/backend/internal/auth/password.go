package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (docs 01 §3.1): memory-hard, interactive-login grade. They apply to the
// login password only. The transaction PIN uses the lighter parameters in pin_pepper.go (ADR-0018).
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

// Bounds for every hash the parser accepts, so a hostile or corrupt stored value cannot make the
// server allocate gigabytes or spin for minutes. The strongest hash in use (m=65536, t=3, p=2) is
// well inside them.
const (
	maxArgonMemoryKiB = 128 * 1024
	maxArgonTime      = 8
	maxArgonThreads   = 8
	minArgonSalt      = 8
	maxArgonSalt      = 64
	minArgonSum       = 16
	maxArgonSum       = 64
)

var ErrPasswordMismatch = errors.New("password mismatch")

// hashSlots bounds concurrent argon2 derivations to the CPU count. Each one
// needs 64 MiB and a full core; without a bound, 50 concurrent PIN confirms
// needed ~3.2 GB and starved the CPUs (load test 2026-11-24, P2). The strong
// parameters are unchanged; excess requests wait their turn instead.
var (
	hashSlots    = make(chan struct{}, max(1, runtime.NumCPU()))
	hashInFlight atomic.Int32
	hashPeak     atomic.Int32 // highest concurrency seen; read by tests
)

// deriveTap is a test hook that sees the parameters of every derivation. nil in production.
var deriveTap func(t, m uint32, p uint8)

// deriveKey is the only place argon2 runs, for passwords and PINs alike, so every derivation
// goes through the concurrency semaphore.
func deriveKey(password, salt []byte, t, m uint32, p uint8, n uint32) []byte {
	hashSlots <- struct{}{}
	cur := hashInFlight.Add(1)
	for {
		peak := hashPeak.Load()
		if cur <= peak || hashPeak.CompareAndSwap(peak, cur) {
			break
		}
	}
	defer func() {
		hashInFlight.Add(-1)
		<-hashSlots
	}()
	if tap := deriveTap; tap != nil {
		tap(t, m, p)
	}
	return argon2.IDKey(password, salt, t, m, p, n)
}

// HashPassword returns an encoded argon2id hash: $argon2id$v=19$m=...,t=...,p=...$salt$key
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := deriveKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// pepperIDPattern is the allowed shape of a pepper key id (same rule as the config).
var pepperIDPattern = regexp.MustCompile(`^[a-z0-9-]{1,16}$`)

// argonHash is a parsed, bounds-checked stored hash. hasK is true for the ADR-0018 format
// (m,t,p,k) and false for the legacy format (m,t,p).
type argonHash struct {
	m, t, p uint32
	keyID   string
	hasK    bool
	salt    []byte
	sum     []byte
}

var errMalformedHash = errors.New("malformed password hash")

func parseUint32(s string) (uint32, bool) {
	// ParseUint refuses a sign, spaces and underscores (base 10), which is what we want.
	v, err := strconv.ParseUint(s, 10, 32)
	return uint32(v), err == nil
}

// parseArgonHash parses $argon2id$v=19$m=..,t=..,p=..[,k=..]$salt$sum strictly: fixed field order,
// no unknown, repeated or missing fields, parameters inside the bounds, sizes inside the bounds.
// Nothing is allocated from the stored numbers before they have been checked.
func parseArgonHash(encoded string) (argonHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argonHash{}, errMalformedHash
	}
	fields := strings.Split(parts[3], ",")
	if len(fields) != 3 && len(fields) != 4 {
		return argonHash{}, errMalformedHash
	}
	var h argonHash
	var ok bool
	for i, want := range []string{"m=", "t=", "p="} {
		if !strings.HasPrefix(fields[i], want) {
			return argonHash{}, errMalformedHash
		}
		var v uint32
		if v, ok = parseUint32(fields[i][len(want):]); !ok {
			return argonHash{}, errMalformedHash
		}
		switch i {
		case 0:
			h.m = v
		case 1:
			h.t = v
		case 2:
			h.p = v
		}
	}
	if len(fields) == 4 {
		id, found := strings.CutPrefix(fields[3], "k=")
		if !found || !pepperIDPattern.MatchString(id) {
			return argonHash{}, errMalformedHash
		}
		h.hasK, h.keyID = true, id
	}
	if h.t < 1 || h.t > maxArgonTime || h.p < 1 || h.p > maxArgonThreads ||
		h.m < 8*h.p || h.m > maxArgonMemoryKiB {
		return argonHash{}, errMalformedHash
	}
	var err error
	if h.salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil || len(h.salt) < minArgonSalt || len(h.salt) > maxArgonSalt {
		return argonHash{}, errMalformedHash
	}
	if h.sum, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil || len(h.sum) < minArgonSum || len(h.sum) > maxArgonSum {
		return argonHash{}, errMalformedHash
	}
	return h, nil
}

// VerifyPassword compares a candidate against an encoded hash in constant time. It refuses a PIN
// hash (one that carries k=): a PIN hash is only ever checked through PepperSet.VerifyPINHash.
func VerifyPassword(encoded, password string) error {
	h, err := parseArgonHash(encoded)
	if err != nil || h.hasK {
		return errMalformedHash
	}
	got := deriveKey([]byte(password), h.salt, h.t, h.m, uint8(h.p), uint32(len(h.sum)))
	if subtle.ConstantTimeCompare(got, h.sum) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}
