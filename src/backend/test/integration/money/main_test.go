package money

import (
	"os"
	"testing"

	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

// These tests pump the shared outbox: serialize against other pumping packages and retire
// stale rows first (see testutil.RunSerialized).
func TestMain(m *testing.M) {
	os.Exit(testutil.RunSerialized(m))
}
