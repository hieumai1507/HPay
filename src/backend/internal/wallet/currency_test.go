package wallet

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// AC-1: the currency string is defined once. No other non-test Go file under src/backend may write
// a quoted VND or USD (the SQL defaults are not Go).
func TestCurrency_NoLiteralOutsideConstant(t *testing.T) {
	if DefaultCurrency != "USD" {
		t.Fatalf("DefaultCurrency = %q, want USD", DefaultCurrency)
	}
	lit := regexp.MustCompile(`"(VND|USD)"`)
	root := filepath.Join("..", "..")
	var bad []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		if strings.Contains(filepath.ToSlash(p), "/store/gen/") || filepath.Base(p) == "currency.go" {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr == nil && lit.Match(b) {
			bad = append(bad, filepath.ToSlash(p))
		}
		return nil
	})
	if len(bad) > 0 {
		t.Fatalf("currency literal outside wallet.DefaultCurrency in: %v", bad)
	}
}