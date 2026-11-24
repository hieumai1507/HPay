package auth

// ADR-0018 / spec w5-pin-pepper: the peppered PIN hash, its parser and the rehash decision.
// Pure functions, no database. Cases TC-PEP-006 to TC-PEP-016 and TC-PEP-028.

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/argon2"
)

func testPepper(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func mustSet(t *testing.T, cur PepperKey, old ...PepperKey) *PepperSet {
	t.Helper()
	ps, err := NewPepperSet(cur, old...)
	if err != nil {
		t.Fatalf("NewPepperSet: %v", err)
	}
	return ps
}

// handHash builds a peppered hash with arbitrary parameters, independent of the code under test.
func handHash(key []byte, id string, tt, m uint32, p uint8, pin string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	sum := argon2.IDKey(pinMAC(key, pin), salt, tt, m, p, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d,k=%s$%s$%s", m, tt, p, id,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(sum))
}

// TC-PEP-006
func TestPinHash_RoundTripFormat(t *testing.T) {
	ps := mustSet(t, PepperKey{"p1", testPepper(1)})
	h, err := ps.HashPIN("482915")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^\$argon2id\$v=19\$m=19456,t=2,p=1,k=p1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)
	if !re.MatchString(h) {
		t.Fatalf("hash does not have the ADR-0018 format: %q", h)
	}
	if strings.Contains(h, "482915") {
		t.Fatal("hash contains the PIN")
	}
	if err := ps.VerifyPINHash(h, "482915"); err != nil {
		t.Fatalf("right PIN must verify: %v", err)
	}
	if ps.NeedsRehash(h) {
		t.Fatal("a fresh hash must not need a rehash")
	}
	h2, _ := ps.HashPIN("482915")
	if h == h2 {
		t.Fatal("two hashes of one PIN must differ (random salt)")
	}
}

// TC-PEP-007
func TestPinHash_WrongPinWrongPepperWrongKeyIDFail(t *testing.T) {
	a := mustSet(t, PepperKey{"p1", testPepper(1)})
	h, err := a.HashPIN("482915")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.VerifyPINHash(h, "482916"); !errors.Is(err, ErrPINHashMismatch) {
		t.Fatalf("wrong PIN: want ErrPINHashMismatch, got %v", err)
	}
	b := mustSet(t, PepperKey{"p1", testPepper(2)})
	if err := b.VerifyPINHash(h, "482915"); !errors.Is(err, ErrPINHashMismatch) {
		t.Fatalf("wrong pepper under the same id: want ErrPINHashMismatch, got %v", err)
	}
	// the hash is labelled p9 but was made with pepper 1; the set holds a different pepper under p9
	mislabelled := handHash(testPepper(1), "p9", 2, 19456, 1, "482915")
	c := mustSet(t, PepperKey{"p1", testPepper(1)}, PepperKey{"p9", testPepper(9)})
	if err := c.VerifyPINHash(mislabelled, "482915"); !errors.Is(err, ErrPINHashMismatch) {
		t.Fatalf("wrong key id: want ErrPINHashMismatch, got %v", err)
	}
}

const (
	zSalt = "AAAAAAAAAAAAAAAAAAAAAA"                      // 16 bytes
	zSum  = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 32 bytes
)

func shaped(version, params, salt, sum string) string {
	return "$argon2id$" + version + "$" + params + "$" + salt + "$" + sum
}

// TC-PEP-008
func TestPinHash_ParserAcceptsKRefusesUnknownFields(t *testing.T) {
	h, err := parseArgonHash(shaped("v=19", "m=19456,t=2,p=1,k=p1", zSalt, zSum))
	if err != nil {
		t.Fatalf("k= must be accepted: %v", err)
	}
	if !h.hasK || h.keyID != "p1" || h.m != 19456 || h.t != 2 || h.p != 1 {
		t.Fatalf("parsed wrong: %+v", h)
	}
	legacy, err := parseArgonHash(shaped("v=19", "m=65536,t=3,p=2", zSalt, zSum))
	if err != nil || legacy.hasK {
		t.Fatalf("legacy hash must parse without k: %v %+v", err, legacy)
	}

	bad := map[string]string{
		"unknown field":    shaped("v=19", "m=19456,t=2,p=1,x=1", zSalt, zSum),
		"unknown after k":  shaped("v=19", "m=19456,t=2,p=1,k=p1,x=1", zSalt, zSum),
		"duplicate k":      shaped("v=19", "m=19456,t=2,p=1,k=p1,k=p2", zSalt, zSum),
		"missing p":        shaped("v=19", "m=19456,t=2,k=p1", zSalt, zSum),
		"misordered":       shaped("v=19", "t=2,m=19456,p=1", zSalt, zSum),
		"k first":          shaped("v=19", "k=p1,m=19456,t=2,p=1", zSalt, zSum),
		"empty k":          shaped("v=19", "m=19456,t=2,p=1,k=", zSalt, zSum),
		"uppercase key id": shaped("v=19", "m=19456,t=2,p=1,k=P1", zSalt, zSum),
		"key id too long":  shaped("v=19", "m=19456,t=2,p=1,k=abcdefghijklmnopq", zSalt, zSum),
		"trailing comma":   shaped("v=19", "m=19456,t=2,p=1,", zSalt, zSum),
		"signed number":    shaped("v=19", "m=+19456,t=2,p=1", zSalt, zSum),
		"wrong version":    shaped("v=18", "m=19456,t=2,p=1,k=p1", zSalt, zSum),
		"not argon2id":     "$argon2i$v=19$m=19456,t=2,p=1,k=p1$" + zSalt + "$" + zSum,
		"bad salt base64":  shaped("v=19", "m=19456,t=2,p=1,k=p1", "!!!!", zSum),
		"salt too short":   shaped("v=19", "m=19456,t=2,p=1,k=p1", "AAAA", zSum),
		"sum too short":    shaped("v=19", "m=19456,t=2,p=1,k=p1", zSalt, "AAAA"),
		"extra segment":    shaped("v=19", "m=19456,t=2,p=1,k=p1", zSalt, zSum) + "$x",
		"empty":            "",
		"plain text":       "482915",
	}
	for name, s := range bad {
		if _, err := parseArgonHash(s); err == nil {
			t.Errorf("%s must be refused: %q", name, s)
		}
	}
}

// TC-PEP-009
func TestPinHash_ParserRefusesAbsurdParameters(t *testing.T) {
	ok := []string{"m=131072,t=8,p=8", "m=8,t=1,p=1", "m=19456,t=2,p=1,k=p1", "m=65536,t=3,p=2"}
	for _, p := range ok {
		if _, err := parseArgonHash(shaped("v=19", p, zSalt, zSum)); err != nil {
			t.Errorf("%s must parse: %v", p, err)
		}
	}
	bad := []string{
		"m=4294967295,t=3,p=2", "m=131073,t=2,p=1", "m=1048576,t=2,p=1",
		"m=19456,t=9,p=1", "m=19456,t=4294967295,p=1", "m=19456,t=4294967296,p=1",
		"m=19456,t=2,p=9", "m=19456,t=2,p=256", "m=19456,t=2,p=4294967297",
		"m=19456,t=0,p=1", "m=19456,t=2,p=0", "m=0,t=2,p=1", "m=7,t=2,p=1", "m=15,t=2,p=2",
	}
	for _, p := range bad {
		if _, err := parseArgonHash(shaped("v=19", p, zSalt, zSum)); err == nil {
			t.Errorf("%s must be refused", p)
		}
	}
	// a hostile hash must be refused before anything is derived
	var calls int
	deriveTap = func(uint32, uint32, uint8) { calls++ }
	defer func() { deriveTap = nil }()
	ps := mustSet(t, PepperKey{"p1", testPepper(1)})
	_ = ps.VerifyPINHash(shaped("v=19", "m=4294967295,t=3,p=2,k=p1", zSalt, zSum), "482915")
	_ = ps.VerifyPINHash(shaped("v=19", "m=4294967295,t=3,p=2", zSalt, zSum), "482915")
	if calls > 2 {
		t.Fatalf("a refused hash may burn work but must never derive with its own parameters (%d calls)", calls)
	}
}

// TC-PEP-010
func TestPinHash_LegacyHashesStillVerify(t *testing.T) {
	ps := mustSet(t, PepperKey{"p1", testPepper(1)})
	legacy, err := HashPassword("482915")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(legacy, ",k=") {
		t.Fatalf("the login password hash must stay unpeppered: %q", legacy)
	}
	if err := ps.VerifyPINHash(legacy, "482915"); err != nil {
		t.Fatalf("legacy hash must verify on the raw PIN: %v", err)
	}
	if err := ps.VerifyPINHash(legacy, "482916"); !errors.Is(err, ErrPINHashMismatch) {
		t.Fatalf("legacy wrong PIN: want ErrPINHashMismatch, got %v", err)
	}
	if !ps.NeedsRehash(legacy) {
		t.Fatal("a legacy hash must be marked for rehash")
	}
}

// TC-PEP-011
func TestPinHash_DistinctPINsNeverCollide(t *testing.T) {
	key := testPepper(7)
	seen := make(map[[32]byte]int, 1_000_000)
	for i := 0; i < 1_000_000; i++ {
		pin := fmt.Sprintf("%06d", i)
		var k [32]byte
		copy(k[:], pinMAC(key, pin))
		if prev, dup := seen[k]; dup {
			t.Fatalf("PINs %06d and %06d produce the same HMAC", prev, i)
		}
		seen[k] = i
		if bytes.Equal(k[:6], []byte(pin)) {
			t.Fatalf("HMAC output starts with the PIN for %s", pin)
		}
	}
	if len(pinMAC(key, "482915")) != 32 {
		t.Fatal("HMAC-SHA256 output must be 32 bytes")
	}
}

// TC-PEP-012
func TestPinHash_DummyDerivationUsesNewParameters(t *testing.T) {
	ps := mustSet(t, PepperKey{"p1", testPepper(1)})
	type call struct {
		t, m uint32
		p    uint8
	}
	var got []call
	deriveTap = func(tt, m uint32, p uint8) { got = append(got, call{tt, m, p}) }
	defer func() { deriveTap = nil }()
	ps.BurnWork()
	if len(got) != 1 || got[0] != (call{2, 19456, 1}) {
		t.Fatalf("want exactly one derivation with t=2 m=19456 p=1, got %+v", got)
	}
}

// TC-PEP-013
func TestPinHash_UnknownKeyIDFailsCleanly(t *testing.T) {
	ps := mustSet(t, PepperKey{"p2", testPepper(2)})
	h := handHash(testPepper(1), "p0", 2, 19456, 1, "482915")
	err := ps.VerifyPINHash(h, "482915")
	if !errors.Is(err, ErrUnknownPepperID) {
		t.Fatalf("want ErrUnknownPepperID, got %v", err)
	}
	if !strings.Contains(err.Error(), "p0") {
		t.Fatalf("the internal error should name the key id: %v", err)
	}
	if strings.Contains(err.Error(), "482915") || strings.Contains(err.Error(), base64.StdEncoding.EncodeToString(testPepper(1))) {
		t.Fatalf("error leaks a secret: %v", err)
	}
	if !ps.NeedsRehash(h) {
		t.Fatal("a hash under a non-current key must be marked for rehash")
	}
	var calls int
	deriveTap = func(uint32, uint32, uint8) { calls++ }
	defer func() { deriveTap = nil }()
	_ = ps.VerifyPINHash(h, "482915")
	if calls != 1 {
		t.Fatalf("an unusable hash must still cost one derivation, got %d", calls)
	}
}

// TC-PEP-014
func TestPinHash_RehashDecision(t *testing.T) {
	p1, p2 := PepperKey{"p1", testPepper(1)}, PepperKey{"p2", testPepper(2)}
	oldSet := mustSet(t, p1)
	cur := mustSet(t, p2, p1)
	fromP1, _ := oldSet.HashPIN("482915")
	fromP2, _ := cur.HashPIN("482915")
	legacy, _ := HashPassword("482915")
	weaker := handHash(p2.Key, "p2", 1, 8192, 1, "482915")
	stronger := handHash(p2.Key, "p2", 3, 65536, 2, "482915")
	cases := map[string]struct {
		h    string
		want bool
	}{
		"legacy": {legacy, true}, "old key": {fromP1, true}, "current": {fromP2, false},
		"older parameters": {weaker, true}, "different parameters": {stronger, true},
		"garbage": {"nonsense", true},
	}
	for name, c := range cases {
		if got := cur.NeedsRehash(c.h); got != c.want {
			t.Errorf("%s: NeedsRehash = %v, want %v", name, got, c.want)
		}
	}
	if err := cur.VerifyPINHash(fromP1, "482915"); err != nil {
		t.Fatalf("an old key kept in the list must still verify: %v", err)
	}
}

// TC-PEP-015
func TestPinHash_NoPepperFailsClosed(t *testing.T) {
	var ps *PepperSet
	if _, err := ps.HashPIN("482915"); !errors.Is(err, ErrPepperNotConfigured) {
		t.Fatalf("HashPIN without a pepper: %v", err)
	}
	if err := ps.VerifyPINHash(shaped("v=19", "m=19456,t=2,p=1,k=p1", zSalt, zSum), "482915"); !errors.Is(err, ErrPepperNotConfigured) {
		t.Fatalf("VerifyPINHash without a pepper: %v", err)
	}
	if _, err := NewPepperSet(PepperKey{"p1", []byte("short")}); err == nil {
		t.Fatal("a short pepper must be refused")
	}
	if _, err := NewPepperSet(PepperKey{"P1", testPepper(1)}); err == nil {
		t.Fatal("a bad key id must be refused")
	}
	if _, err := NewPepperSet(PepperKey{"p1", testPepper(1)}, PepperKey{"p1", testPepper(2)}); err == nil {
		t.Fatal("a duplicate key id must be refused")
	}
}

// TC-PEP-016
func TestPassword_RefusesPepperedHash(t *testing.T) {
	ps := mustSet(t, PepperKey{"p1", testPepper(1)})
	h, _ := ps.HashPIN("482915")
	err := VerifyPassword(h, "482915")
	if err == nil {
		t.Fatal("VerifyPassword must refuse a peppered PIN hash")
	}
	if errors.Is(err, ErrPasswordMismatch) {
		t.Fatal("a peppered hash must be refused as malformed, not compared")
	}
}

// TC-PEP-028
func TestPinHash_ConcurrencyIsBounded(t *testing.T) {
	ps := mustSet(t, PepperKey{"p1", testPepper(1)})
	h, err := ps.HashPIN("482915")
	if err != nil {
		t.Fatal(err)
	}
	hashPeak.Store(0)
	n := runtime.NumCPU()*4 + 4
	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); errs <- ps.VerifyPINHash(h, "482915") }()
		go func() { defer wg.Done(); ps.BurnWork(); errs <- nil }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("verify under load: %v", err)
		}
	}
	if peak := int(hashPeak.Load()); peak > runtime.NumCPU() || peak < 1 {
		t.Fatalf("peak concurrent derivations %d, CPU count %d", peak, runtime.NumCPU())
	}
}
