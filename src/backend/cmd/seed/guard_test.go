package main

// Slice 3 (BE-H7, ADR-0012): the seeder creates users and admins with known
// passwords, so it must refuse to run against production or a remote database.
// TC-ENV-007.

import "testing"

func TestSeedGuard(t *testing.T) {
	const local = "postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable"
	cases := []struct {
		name        string
		env         string
		url         string
		allowRemote bool
		wantErr     bool
	}{
		{"dev local", "dev", local, false, false},
		{"ci local", "ci", local, false, false},
		{"demo local", "demo", local, false, false},
		{"production refused", "production", local, false, true},
		{"production refused even with flag", "production", local, true, true},
		{"unknown env refused", "staging", local, false, true},
		{"empty env refused", "", local, false, true},
		{"remote host refused", "dev", "postgres://u:p@db.example.com:5432/x", false, true},
		{"neon style host refused", "dev", "postgres://u:p@ep-cool-123.ap-southeast-1.aws.neon.tech/db?sslmode=require", false, true},
		{"remote host allowed with flag", "dev", "postgres://u:p@db.example.com:5432/x", true, false},
		{"127.0.0.1 allowed", "dev", "postgres://u:p@127.0.0.1:5432/x", false, false},
		{"ipv6 loopback allowed", "dev", "postgres://u:p@[::1]:5432/x", false, false},
		{"compose service name allowed", "dev", "postgres://u:p@postgres:5432/x", false, false},
		{"docker host allowed", "dev", "postgres://u:p@host.docker.internal:5432/x", false, false},
		{"missing url refused", "dev", "", false, true},
		{"garbage url refused", "dev", "not a url \x7f", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := checkSeedTarget(c.env, c.url, c.allowRemote)
			if (err != nil) != c.wantErr {
				t.Fatalf("checkSeedTarget(%q, %q, %v): err=%v, wantErr=%v", c.env, c.url, c.allowRemote, err, c.wantErr)
			}
		})
	}
}

func TestSeedGuardNamesTheHostItRefuses(t *testing.T) {
	_, err := checkSeedTarget("dev", "postgres://u:secret@db.example.com:5432/x", false)
	if err == nil {
		t.Fatal("want an error")
	}
	if !contains(err.Error(), "db.example.com") {
		t.Errorf("error should name the host so the operator can see what was refused: %v", err)
	}
	if contains(err.Error(), "secret") {
		t.Errorf("error must not echo the password: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
