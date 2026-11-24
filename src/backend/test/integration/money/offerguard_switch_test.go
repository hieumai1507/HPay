package money

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/offerguard"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// The OfferGuard switch now comes from config (Config.OfferGuardDisabled, refused outside dev and ci).
// In the ci test environment it must still turn the limits off, and the default must keep them on.
func TestOfferGuardSwitch_OffDisablesLimitsOnDefaultKeepsThem(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	sender, _, _ := e.UserWithMoney(t, 2_000_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)
	code := "OFFER_TEST_" + uuid.NewString()[:8]
	if _, err := e.Pool.Exec(ctx, `
		WITH o AS (INSERT INTO offers (code, name, role, level, is_active) VALUES ($1, 'switch test', 'customer', 1, true) RETURNING id)
		INSERT INTO offer_services (offer_id, service_id, is_enabled, min_amount, max_amount_per_txn, daily_limit, monthly_limit, interval_seconds)
		SELECT o.id, s.id, true, 0, 10000, 0, 0, 0 FROM o, services s WHERE s.code = 'P2P_TRANSFER'`, code); err != nil {
		t.Fatal(err)
	}
	e.SetOffer(t, sender, code)
	e.SetPIN(t, sender, testkit.ValidPIN)

	in := func() transfer.EngineRequestInput {
		return transfer.EngineRequestInput{ServiceCode: "P2P_TRANSFER", SenderUserID: sender,
			ReceiverPhone: receiverPhone, Amount: 500_000, Idempotency: uuid.NewString()}
	}
	// 500,000 is above this offer's per-transaction cap of 10,000.
	if _, err := e.Svc.ExecuteEngineRequest(ctx, in()); err == nil {
		t.Fatal("default: the limit must refuse 500,000")
	} else if !errors.Is(err, offerguard.ErrLimitDaily) && !errors.Is(err, offerguard.ErrLimitPerTxn) {
		t.Fatalf("default: want a limit error, got %v", err)
	}
	e.Svc.SetOfferGuardEnabled(false)
	if _, err := e.Svc.ExecuteEngineRequest(ctx, in()); err != nil {
		t.Fatalf("guard switched off (ci): the request must pass, got %v", err)
	}
}
