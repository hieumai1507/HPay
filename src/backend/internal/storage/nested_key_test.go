package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A key with several path segments, as the demo seed uses ("seed/kyc/id-card.png"), must round-trip.
func TestLocal_NestedKeyRoundTrip(t *testing.T) {
	root := t.TempDir()
	l, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "seed", "kyc"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "seed", "kyc", "id-card.png"), []byte("png-bytes"), 0o640); err != nil {
		t.Fatal(err)
	}
	got, err := l.Get(context.Background(), "seed/kyc/id-card.png")
	if err != nil || string(got) != "png-bytes" {
		t.Fatalf("nested key: got %q, %v", got, err)
	}
}

// The root may be given as a relative path (the default is ./data/kyc) or with a trailing slash.
func TestLocal_RelativeAndTrailingSlashRoot(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	for _, root := range []string{"./data/kyc", "data/kyc/", "/" + filepath.ToSlash(filepath.Join(dir, "abs")) + "/"} {
		_ = root
	}
	l, err := NewLocal("./data/kyc")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Put(context.Background(), "a/b.png", []byte("x")); err != nil {
		t.Fatal(err)
	}
	got, err := l.Get(context.Background(), "a/b.png")
	if err != nil || string(got) != "x" {
		t.Fatalf("relative root: got %q, %v", got, err)
	}
}
