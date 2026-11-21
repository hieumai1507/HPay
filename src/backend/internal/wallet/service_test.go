package wallet

import (
	"strings"
	"testing"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// TestEdge_E13_LimitByKycTier: hạn mức per-txn theo tier KYC.
func TestEdge_E13_LimitByKycTier(t *testing.T) {
	cases := []struct {
		kyc   store.KycStatus
		limit int64
	}{
		{store.KycStatusNone, 2_000_000},
		{store.KycStatusRejected, 2_000_000},
		{store.KycStatusPending, 5_000_000},
		{store.KycStatusManualReview, 5_000_000},
		{store.KycStatusAutoApproved, 100_000_000},
		{store.KycStatusVerified, 100_000_000},
	}
	for _, c := range cases {
		if got := TxnLimit(c.kyc); got != c.limit {
			t.Errorf("tier %s: want %d got %d", c.kyc, c.limit, got)
		}
	}
}

func TestNewRefCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		rc := NewRefCode()
		if !strings.HasPrefix(rc, "MP") || len(rc) != 14 {
			t.Fatalf("bad ref code %q", rc)
		}
		if seen[rc] {
			t.Fatalf("duplicate ref code %q", rc)
		}
		seen[rc] = true
	}
}
