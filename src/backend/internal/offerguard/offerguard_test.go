package offerguard_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/offerguard"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

// setupOffer builds an isolated offer package for one user so the assertions do
// not depend on the seeded defaults (which other tests may also touch).
func setupOffer(t *testing.T, pool *pgxpool.Pool, serviceCode string, limits offerLimits) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	userID, _ := testutil.NewUserWithWallet(t, pool)

	var serviceID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM services WHERE code=$1`, serviceCode).Scan(&serviceID); err != nil {
		t.Fatalf("service %s missing — run migrations: %v", serviceCode, err)
	}

	offerCode := fmt.Sprintf("OFFER_TEST_%d", rand.Intn(1_000_000_000))
	var offerID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO offers (code, name, role, level, is_active)
		VALUES ($1, 'Test offer', 'customer', 1, true) RETURNING id`, offerCode).Scan(&offerID); err != nil {
		t.Fatalf("insert offer: %v", err)
	}

	var offerServiceID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO offer_services
		  (offer_id, service_id, is_enabled, min_amount, max_amount_per_txn, daily_limit, monthly_limit, interval_seconds)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		offerID, serviceID, limits.enabled, limits.min, limits.perTxn, limits.daily, limits.monthly, limits.interval,
	).Scan(&offerServiceID); err != nil {
		t.Fatalf("insert offer_service: %v", err)
	}

	for _, tier := range limits.tiers {
		var maxAmt any
		if tier.max > 0 {
			maxAmt = tier.max
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO offer_auth_matrix (offer_service_id, min_amount, max_amount, auth_method)
			VALUES ($1,$2,$3,$4)`, offerServiceID, tier.min, maxAmt, tier.method); err != nil {
			t.Fatalf("insert auth matrix: %v", err)
		}
	}

	if err := store.New(pool).AssignUserOffer(ctx, store.AssignUserOfferParams{
		UserID: userID, OfferID: offerID, AssignedBy: "test",
	}); err != nil {
		t.Fatalf("assign offer: %v", err)
	}
	return userID
}

type authTier struct {
	min, max int64
	method   string
}

type offerLimits struct {
	enabled                     bool
	min, perTxn, daily, monthly int64
	interval                    int32
	tiers                       []authTier
}

func defaultLimits() offerLimits {
	return offerLimits{
		enabled: true, min: 0, perTxn: 40_000, daily: 80_000, monthly: 400_000,
		interval: 0,
		tiers: []authTier{
			{min: 0, max: 8_000, method: "pin"},
			{min: 8_000, max: 20_000, method: "otp"},
			{min: 20_000, max: 0, method: "2fa"}, // 0 => NULL ceiling
		},
	}
}

// G3: the required auth method comes from offer_auth_matrix rows, not from
// thresholds hardcoded in Go.
func TestAuthMatrix_FromDB(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	userID := setupOffer(t, pool, "P2P_TRANSFER", defaultLimits())
	g := offerguard.New(pool)

	cases := []struct {
		amount int64
		want   string
	}{
		{4_000, "pin"},
		{12_000, "otp"},
		{36_000, "2fa"},
	}
	for _, tc := range cases {
		got, err := g.Check(context.Background(), userID, "P2P_TRANSFER", store.TxnTypeP2p, tc.amount)
		if err != nil {
			t.Fatalf("amount %d: unexpected error: %v", tc.amount, err)
		}
		if got.AuthMethod != tc.want {
			t.Errorf("amount %d: auth = %q, want %q", tc.amount, got.AuthMethod, tc.want)
		}
	}
}

// G2: per-transaction ceiling from the offer package.
func TestOfferGuard_LimitPerTxn(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	userID := setupOffer(t, pool, "P2P_TRANSFER", defaultLimits())
	g := offerguard.New(pool)

	if _, err := g.Check(context.Background(), userID, "P2P_TRANSFER", store.TxnTypeP2p, 10_000_001); !errors.Is(err, offerguard.ErrLimitPerTxn) {
		t.Fatalf("over per-txn limit: err = %v, want ErrLimitPerTxn", err)
	}
	if _, err := g.Check(context.Background(), userID, "P2P_TRANSFER", store.TxnTypeP2p, 40_000); err != nil {
		t.Fatalf("at per-txn limit should pass, got %v", err)
	}
}

// G2: a disabled service inside the package is refused.
func TestOfferGuard_ServiceDisabled(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	lim := defaultLimits()
	lim.enabled = false
	userID := setupOffer(t, pool, "P2P_TRANSFER", lim)
	g := offerguard.New(pool)

	if _, err := g.Check(context.Background(), userID, "P2P_TRANSFER", store.TxnTypeP2p, 4); !errors.Is(err, offerguard.ErrServiceDisabled) {
		t.Fatalf("err = %v, want ErrServiceDisabled", err)
	}
}

// G2: amount below the service floor is refused.
func TestOfferGuard_BelowMinimum(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	lim := defaultLimits()
	lim.min = 200
	userID := setupOffer(t, pool, "P2P_TRANSFER", lim)
	g := offerguard.New(pool)

	if _, err := g.Check(context.Background(), userID, "P2P_TRANSFER", store.TxnTypeP2p, 199); !errors.Is(err, offerguard.ErrBelowMin) {
		t.Fatalf("err = %v, want ErrBelowMin", err)
	}
}

// Fail-closed: an unknown service code is never allowed through unmetered.
func TestOfferGuard_UnknownServiceRefused(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	userID := setupOffer(t, pool, "P2P_TRANSFER", defaultLimits())
	g := offerguard.New(pool)

	if _, err := g.Check(context.Background(), userID, "NO_SUCH_SERVICE", store.TxnTypeP2p, 4); !errors.Is(err, offerguard.ErrServiceNotInOffer) {
		t.Fatalf("err = %v, want ErrServiceNotInOffer", err)
	}
}
