package config

// ADR-0018 / spec w5-pin-pepper: PIN_PEPPER, PIN_PEPPER_ID and PIN_PEPPER_OLD.
// Cases TC-PEP-001 to TC-PEP-005. Same shape as TestConfig_TOTPKeyRequiredOutsideDev.

import (
	"encoding/base64"
	"strings"
	"testing"
)

const (
	goodTOTP   = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	pepperA    = "QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE=" // 32 x 'A'
	pepperB    = "QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI=" // 32 x 'B'
	pepperC    = "Q0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0M=" // 32 x 'C'
	devPepper  = DevPINPepper
	shortValue = "c2hvcnQ=" // 5 bytes
)

func pepperEnv(t *testing.T, env, pepper, id, old string) {
	t.Helper()
	base(t)
	t.Setenv("APP_ENV", env)
	t.Setenv("TOTP_ENCRYPTION_KEY", goodTOTP)
	t.Setenv("PIN_PEPPER", pepper)
	t.Setenv("PIN_PEPPER_ID", id)
	t.Setenv("PIN_PEPPER_OLD", old)
}

// TC-PEP-001, TC-PEP-002
func TestConfig_PinPepperRequiredOutsideDev(t *testing.T) {
	for _, env := range []string{"production", "demo"} {
		pepperEnv(t, env, "", "", "")
		if _, err := Load("api"); err == nil || !strings.Contains(err.Error(), "PIN_PEPPER") {
			t.Errorf("%s without a pepper must fail naming PIN_PEPPER, got %v", env, err)
		}
		pepperEnv(t, env, pepperA, "", "")
		cfg, err := Load("api")
		if err != nil {
			t.Fatalf("%s with a good pepper: %v", env, err)
		}
		if len(cfg.PINPepper) != 32 || cfg.PINPepperID != "p1" || len(cfg.PINPepperOld) != 0 {
			t.Errorf("%s: want a 32 byte pepper, id p1 and no old keys, got %d bytes, id %q, %d old", env, len(cfg.PINPepper), cfg.PINPepperID, len(cfg.PINPepperOld))
		}
	}
	for _, env := range []string{"dev", "ci"} {
		pepperEnv(t, env, "", "", "")
		cfg, err := Load("api")
		if err != nil {
			t.Fatalf("%s must not need a pepper: %v", env, err)
		}
		if len(cfg.PINPepper) != 32 || cfg.PINPepperID != "p1" {
			t.Errorf("%s gets a fixed 32-byte test pepper with id p1, got %d bytes and id %q", env, len(cfg.PINPepper), cfg.PINPepperID)
		}
	}
}

// TC-PEP-003
func TestConfig_PinPepperMalformedRefused(t *testing.T) {
	cases := []struct {
		name, pepper, id, old, want string
	}{
		{"not base64", "not base64 !!", "", "", "PIN_PEPPER"},
		{"short", shortValue, "", "", "32"},
		{"long", base64.StdEncoding.EncodeToString(make([]byte, 33)), "", "", "32"},
		{"id uppercase", pepperA, "P1", "", "PIN_PEPPER_ID"},
		{"id too long", pepperA, "abcdefghijklmnopq", "", "PIN_PEPPER_ID"},
		{"id with underscore", pepperA, "p_1", "", "PIN_PEPPER_ID"},
		{"old without colon", pepperA, "p2", pepperB, "PIN_PEPPER_OLD"},
		{"old empty id", pepperA, "p2", ":" + pepperB, "PIN_PEPPER_OLD"},
		{"old bad id", pepperA, "p2", "P1:" + pepperB, "PIN_PEPPER_OLD"},
		{"old not base64", pepperA, "p2", "p1:not base64 !!", "PIN_PEPPER_OLD"},
		{"old short", pepperA, "p2", "p1:" + shortValue, "PIN_PEPPER_OLD"},
		{"old duplicate id", pepperA, "p3", "p1:" + pepperB + ",p1:" + pepperC, "PIN_PEPPER_OLD"},
		{"old equals current id", pepperA, "p2", "p2:" + pepperB, "PIN_PEPPER_OLD"},
		{"old empty entry", pepperA, "p2", "p1:" + pepperB + ",,", "PIN_PEPPER_OLD"},
	}
	for _, c := range cases {
		for _, env := range []string{"production", "dev"} {
			pepperEnv(t, env, c.pepper, c.id, c.old)
			_, err := Load("api")
			if err == nil {
				t.Errorf("%s/%s must refuse startup", env, c.name)
				continue
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s/%s: error should mention %q: %v", env, c.name, c.want, err)
			}
			for _, secret := range []string{c.pepper, pepperA, pepperB, pepperC} {
				if secret != "" && strings.Contains(err.Error(), secret) {
					t.Errorf("%s/%s: the error leaks a key value: %v", env, c.name, err)
				}
			}
		}
	}
}

// TC-PEP-004
func TestConfig_PinPepperOldAndIDParsed(t *testing.T) {
	pepperEnv(t, "production", pepperA, "p2", "p1:"+pepperB+", p0:"+pepperC)
	cfg, err := Load("api")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PINPepperID != "p2" || len(cfg.PINPepper) != 32 {
		t.Fatalf("current key wrong: id %q, %d bytes", cfg.PINPepperID, len(cfg.PINPepper))
	}
	if len(cfg.PINPepperOld) != 2 || cfg.PINPepperOld[0].ID != "p1" || cfg.PINPepperOld[1].ID != "p0" ||
		len(cfg.PINPepperOld[0].Key) != 32 || cfg.PINPepperOld[0].Key[0] != 'B' || cfg.PINPepperOld[1].Key[0] != 'C' {
		t.Fatalf("old keys wrong: %+v", cfg.PINPepperOld)
	}
}

// TC-PEP-005: the public dev and ci value must never protect real PINs.
func TestConfig_PinPepperDevValueRefusedInProduction(t *testing.T) {
	enc := base64.StdEncoding.EncodeToString([]byte(devPepper))
	for _, env := range []string{"production", "demo"} {
		pepperEnv(t, env, enc, "", "")
		if _, err := Load("api"); err == nil || !strings.Contains(err.Error(), "PIN_PEPPER") {
			t.Errorf("%s must refuse the dev test pepper, got %v", env, err)
		}
		pepperEnv(t, env, pepperA, "p2", "p1:"+enc)
		if _, err := Load("api"); err == nil || !strings.Contains(err.Error(), "PIN_PEPPER") {
			t.Errorf("%s must refuse the dev test pepper in PIN_PEPPER_OLD, got %v", env, err)
		}
	}
	pepperEnv(t, "dev", base64.StdEncoding.EncodeToString([]byte(devPepper)), "", "")
	if _, err := Load("api"); err != nil {
		t.Errorf("dev may use the dev value: %v", err)
	}
}
