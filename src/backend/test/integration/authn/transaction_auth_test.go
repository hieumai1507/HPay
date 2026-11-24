package authn

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TC-AUT-001: confirm and verify never reveal another user's transaction.
func TestConfirm_OtherUsersTransactionIsNotFound(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	owner, _, _ := e.UserWithMoney(t, 2_000_000)
	_, _, rcvPhone := e.UserWithMoney(t, 0)
	stranger, _, _ := e.UserWithMoney(t, 0)
	res := e.RequestP2P(t, owner, rcvPhone, 100_000)

	_, errOther := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: stranger, PIN: testkit.ValidPIN})
	_, errMissing := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: uuid.New(), SenderUserID: stranger, PIN: testkit.ValidPIN})
	if !errors.Is(errOther, transfer.ErrTxnNotFound) || !errors.Is(errMissing, transfer.ErrTxnNotFound) {
		t.Fatalf("confirm: other user's id and unknown id must both be ErrTxnNotFound, got %v / %v", errOther, errMissing)
	}
	_, errVerify := e.Svc.ExecuteEngineVerify(ctx, res.TransactionID, stranger, "123456")
	if !errors.Is(errVerify, transfer.ErrTxnNotFound) {
		t.Fatalf("verify: want ErrTxnNotFound, got %v", errVerify)
	}
}

// TC-AUT-002: a request that needs a PIN does not move money or start the workflow.
func TestRequest_PinTransferWaitsForAuth(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, sw, _ := e.UserWithMoney(t, 2_000_000)
	_, rw, rcvPhone := e.UserWithMoney(t, 0)
	res := e.RequestP2P(t, sender, rcvPhone, 100_000)
	e.Drain(t)

	if res.AuthRequired != "pin" {
		t.Fatalf("setup: expected the offer to require a pin for 100,000, got %q", res.AuthRequired)
	}
	if res.Status != "awaiting_auth" {
		t.Fatalf("response status = %q, want awaiting_auth", res.Status)
	}
	if got := e.TxnStatus(t, res.RefCode); got != "awaiting_auth" {
		t.Fatalf("stored status = %q, want awaiting_auth", got)
	}
	if n := e.InstanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow instances = %d, want 0 before authentication", n)
	}
	var entries int
	_ = e.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1`, res.TransactionID).Scan(&entries)
	if entries != 0 {
		t.Fatalf("ledger entries before authentication = %d, want 0", entries)
	}
	if got := testutil.WalletBalance(t, e.Pool, sw); got != 2_000_000 {
		t.Errorf("sender balance changed to %d", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, rw); got != 0 {
		t.Errorf("receiver balance changed to %d", got)
	}
}

// TC-AUT-003 and TC-AUT-013: a wrong or missing PIN never starts the workflow; the right one does, once.
func TestConfirm_WrongPinDoesNotStartWorkflow(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	sender, sw, _ := e.UserWithMoney(t, 2_000_000)
	e.SetPIN(t, sender, testkit.ValidPIN)
	_, rw, rcvPhone := e.UserWithMoney(t, 0)
	res := e.RequestP2P(t, sender, rcvPhone, 100_000)

	for _, pin := range []string{"908172", ""} {
		_, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: pin})
		if !errors.Is(err, auth.ErrPINIncorrect) {
			t.Fatalf("PIN %q: want ErrPINIncorrect, got %v", pin, err)
		}
	}
	e.Drain(t)
	if n := e.InstanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow started after wrong PINs: %d instances", n)
	}
	var attempts int
	_ = e.Pool.QueryRow(ctx, `SELECT pin_failed_attempts FROM users WHERE id=$1`, sender).Scan(&attempts)
	if attempts != 2 {
		t.Errorf("failed attempts = %d, want 2", attempts)
	}

	out, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN})
	if err != nil {
		t.Fatalf("correct PIN: %v", err)
	}
	e.Drain(t)
	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("after confirm: status %q, want completed (response status %q)", got, out.Status)
	}
	if got := testutil.WalletBalance(t, e.Pool, rw); got != 100_000 {
		t.Errorf("receiver balance = %d, want 100000", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, sw); got != 2_000_000-100_000-res.Fee {
		t.Errorf("sender balance = %d, want %d", got, 2_000_000-100_000-res.Fee)
	}

	// TC-AUT-013: confirming again returns the state and does not start a second workflow.
	if _, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN}); err != nil {
		t.Fatalf("second confirm: %v", err)
	}
	e.Drain(t)
	if n := e.InstanceCount(t, res.TransactionID); n != 1 {
		t.Fatalf("workflow instances after a second confirm = %d, want 1", n)
	}
	if got := testutil.WalletBalance(t, e.Pool, rw); got != 100_000 {
		t.Errorf("receiver balance after second confirm = %d, want 100000", got)
	}
}

// TC-AUT-005: concurrent confirms start the workflow once and move money once.
func TestConfirm_ConcurrentStartsWorkflowOnce(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	sender, _, _ := e.UserWithMoney(t, 2_000_000)
	e.SetPIN(t, sender, testkit.ValidPIN)
	_, rw, rcvPhone := e.UserWithMoney(t, 0)
	res := e.RequestP2P(t, sender, rcvPhone, 100_000)

	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN})
		}(i)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("confirm %d: %v", i, err)
		}
	}
	e.Drain(t)
	if n := e.InstanceCount(t, res.TransactionID); n != 1 {
		t.Fatalf("workflow instances = %d, want exactly 1", n)
	}
	if got := testutil.WalletBalance(t, e.Pool, rw); got != 100_000 {
		t.Errorf("receiver balance = %d, want 100000 (moved once)", got)
	}
}

// TC-AUT-006: a user without a PIN cannot confirm a pin transaction.
func TestConfirm_NoPinSet(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, _, _ := e.UserWithMoney(t, 2_000_000)
	_, _, rcvPhone := e.UserWithMoney(t, 0)
	res := e.RequestP2P(t, sender, rcvPhone, 100_000)
	_, err := e.Svc.ExecuteEngineConfirm(context.Background(), transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN})
	if !errors.Is(err, auth.ErrPINNotSet) {
		t.Fatalf("want ErrPINNotSet, got %v", err)
	}
	if n := e.InstanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow started without a PIN: %d", n)
	}
}

// AC-4 through the confirm path: five wrong PINs lock further attempts, even a correct one.
func TestConfirm_LockoutRefusesCorrectPin(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	sender, _, _ := e.UserWithMoney(t, 2_000_000)
	e.SetPIN(t, sender, testkit.ValidPIN)
	_, _, rcvPhone := e.UserWithMoney(t, 0)
	res := e.RequestP2P(t, sender, rcvPhone, 100_000)
	for i := 0; i < 5; i++ {
		_, _ = e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: "908172"})
	}
	_, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN})
	if !errors.Is(err, auth.ErrPINLocked) {
		t.Fatalf("correct PIN during the lock: want ErrPINLocked, got %v", err)
	}
	if n := e.InstanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow started during a lock: %d", n)
	}
}

// TC-AUT-007 (updated by ADR-0017): the 2fa band is no longer "unavailable", but it still fails
// closed: without an enrolled second factor the request is refused and nothing is held. The
// enrolled path is covered by totp_flow_test.go (TC-2FA-010 to 015).
func TestRequest_TwoFactorBandIsRefused(t *testing.T) {
	e := testkit.NewEnv(t, "")
	// The 2fa band starts at 50,000,000. Fund through the ledger path in two steps.
	sender, sw, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, sender, "OFFER_CUST_L2") // 60M is above the 2fa threshold and under the L2 cap
	for i := 0; i < 2; i++ {
		testkit.Fund(t, e.Pool, sender, sw, 40_000_000)
	}
	_, _, rcvPhone := e.UserWithMoney(t, 0)
	// funding posts p2p rows initiated by the sender, which the interval throttle would count
	testkit.AgeUserTransactions(t, e, sender, "1 hour")
	_, err := e.Svc.ExecuteEngineRequest(context.Background(), transfer.EngineRequestInput{
		ServiceCode: "P2P_TRANSFER", SenderUserID: sender, ReceiverPhone: rcvPhone,
		Amount: 60_000_000, Idempotency: wallet.NewRefCode(),
	})
	if !errors.Is(err, auth.ErrTOTPNotEnrolled) {
		t.Fatalf("a request that needs 2fa without an enrolled factor must be refused with ErrTOTPNotEnrolled, got %v", err)
	}
}

// TC-AUT-008: an unconfirmed request expires without moving money.
func TestAwaitingAuth_ExpiresWithoutMovingMoney(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	sender, sw, _ := e.UserWithMoney(t, 2_000_000)
	_, _, rcvPhone := e.UserWithMoney(t, 0)
	res := e.RequestP2P(t, sender, rcvPhone, 100_000)
	if _, err := e.Pool.Exec(ctx, `UPDATE transactions SET created_at = now() - interval '11 minutes' WHERE id=$1`, res.TransactionID); err != nil {
		t.Fatal(err)
	}
	n, err := e.Svc.ExpireAwaitingAuth(ctx, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expired %d, want at least 1", n)
	}
	if got := e.TxnStatus(t, res.RefCode); got != "failed" {
		t.Fatalf("status = %q, want failed", got)
	}
	var reason string
	_ = e.Pool.QueryRow(ctx, `SELECT coalesce(failure_reason,'') FROM transactions WHERE id=$1`, res.TransactionID).Scan(&reason)
	if reason != "auth_expired" {
		t.Errorf("failure_reason = %q, want auth_expired", reason)
	}
	if got := testutil.WalletBalance(t, e.Pool, sw); got != 2_000_000 {
		t.Errorf("sender balance changed to %d", got)
	}
	// a confirm after expiry must not resurrect it
	e.SetPIN(t, sender, testkit.ValidPIN)
	if _, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN}); err == nil {
		t.Fatal("confirming an expired request must fail")
	}
	if n := e.InstanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow started for an expired request: %d", n)
	}
}

// TC-AUT-011: unconfirmed requests do not consume the daily quota.
func TestOfferGuard_AwaitingAuthDoesNotConsumeQuota(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	sender, _, _ := e.UserWithMoney(t, 1_000_000)
	_, _, rcvPhone := e.UserWithMoney(t, 0)
	e.SetPIN(t, sender, testkit.ValidPIN)
	for i := 0; i < 3; i++ {
		_ = e.RequestP2P(t, sender, rcvPhone, 100_000)
		// the interval throttle would reject an immediate second attempt; age the previous row
		testkit.AgeUserTransactions(t, e, sender, "1 minute")
	}
	var sum int64
	if err := e.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount + fee), 0)::bigint FROM transactions WHERE initiated_by=$1 AND type='p2p' AND status NOT IN ('failed','reversed','awaiting_auth')`, sender).Scan(&sum); err != nil {
		t.Fatal(err)
	}
	if sum != 0 {
		t.Fatalf("quota sum with only awaiting_auth requests = %d, want 0", sum)
	}
}
