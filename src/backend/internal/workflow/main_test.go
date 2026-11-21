package workflow

import (
	"os"
	"testing"

	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

// This package pumps the shared outbox — serialize against other pumping
// packages and retire stale rows first (see testutil.RunSerialized).
func TestMain(m *testing.M) {
	os.Exit(testutil.RunSerialized(m))
}
