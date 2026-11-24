package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (docs 01 §3.1): memory-hard, interactive-login grade.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
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

// VerifyPassword compares a candidate against an encoded hash in constant time.
func VerifyPassword(encoded, password string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return errors.New("malformed password hash")
	}
	var version, memory, timeCost, threads int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return err
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return err
	}
	got := deriveKey([]byte(password), salt, uint32(timeCost), uint32(memory), uint8(threads), uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}
