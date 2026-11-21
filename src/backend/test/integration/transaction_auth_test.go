package integration

// Wave 3 slice: transaction authentication and ownership (BE-C2, ADR-0008, ADR-0014).
// Cases map to docs/specs/w3-transaction-auth/test-cases.md (TC-AUT-001..013).

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
)

const testPIN = "482915"

// setPIN gives a user a real PIN through the same code path the API uses.
func (e *env) setPIN(t *testing.T, userID uuid.UUID, pin string) {
	t.Helper()
	pw := "S3cret!password"
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(context.Background(), `UPDATE users SET password_hash=$2 WHERE id=$1`, userID, h); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPIN(context.Background(), userID, pw, pin); err != nil {
		t.Fatalf("set PIN: %v", err)
	}
}

func (e *env) requestP2P(t *testing.T, sender uuid.UUID, receiverPhone string, amount int64) transfer.EngineRequestResult {
	t.Helper()
	res, err := e.svc.ExecuteEngineRequest(context.Background(), transfer.EngineRequestInput{
		ServiceCode: "P2P_TRANSFER", SenderUserID: sender, ReceiverPhone: receiverPhone,
		Amount: amount, Idempotency: wallet.NewRefCode(),
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return res
}

func (e *env) instanceCount(t *testing.T, txnID uuid.UUID) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM workflow_instances WHERE transaction_id=$1`, txnID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TC-AUT-001: confirm and verify never reveal another user's transaction.
func TestConfirm_OtherUsersTransactionIsNotFound(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	owner, _, _ := e.userWithMoney(t, 2_000_000)
	_, _, rcvPhone := e.userWithMoney(t, 0)
	stranger, _, _ := e.userWithMoney(t, 0)
	res := e.requestP2P(t, owner, rcvPhone, 100_000)

	_, errOther := e.svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: stranger, PIN: testPIN})
	_, errMissing := e.svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: uuid.New(), SenderUserID: stranger, PIN: testPIN})
	if !errors.Is(errOther, transfer.ErrTxnNotFound) || !errors.Is(errMissing, transfer.ErrTxnNotFound) {
		t.Fatalf("confirm: other user's id and unknown id must both be ErrTxnNotFound, got %v / %v", errOther, errMissing)
	}
	_, errVerify := e.svc.ExecuteEngineVerify(ctx, res.TransactionID, stranger, "123456")
	if !errors.Is(errVerify, transfer.ErrTxnNotFound) {
		t.Fatalf("verify: want ErrTxnNotFound, got %v", errVerify)
	}
}

// TC-AUT-002: a request that needs a PIN does not move money or start the workflow.
func TestRequest_PinTransferWaitsForAuth(t *testing.T) {
	e := newEnv(t, "")
	sender, sw, _ := e.userWithMoney(t, 2_000_000)
	_, rw, rcvPhone := e.userWithMoney(t, 0)
	res := e.requestP2P(t, sender, rcvPhone, 100_000)
	e.drain(t)

	if res.AuthRequired != "pin" {
		t.Fatalf("setup: expected the offer to require a pin for 100,000, got %q", res.AuthRequired)
	}
	if res.Status != "awaiting_auth" {
		t.Fatalf("response status = %q, want awaiting_auth", res.Status)
	}
	if got := e.txnStatus(t, res.RefCode); got != "awaiting_auth" {
		t.Fatalf("stored status = %q, want awaiting_auth", got)
	}
	if n := e.instanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow instances = %d, want 0 before authentication", n)
	}
	var entries int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1`, res.TransactionID).Scan(&entries)
	if entries != 0 {
		t.Fatalf("ledger entries before authentication = %d, want 0", entries)
	}
	if got := testutil.WalletBalance(t, e.pool, sw); got != 2_000_000 {
		t.Errorf("sender balance changed to %d", got)
	}
	if got := testutil.WalletBalance(t, e.pool, rw); got != 0 {
		t.Errorf("receiver balance changed to %d", got)
	}
}

// TC-AUT-003 and TC-AUT-013: a wrong or missing PIN never starts the workflow; the right one does, once.
func TestConfirm_WrongPinDoesNotStartWorkflow(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	sender, sw, _ := e.userWithMoney(t, 2_000_000)
	e.setPIN(t, sender, testPIN)
	_, rw, rcvPhone := e.userWithMoney(t, 0)
	res := e.requestP2P(t, sender, rcvPhone, 100_000)

	for _, pin := range []string{"908172", ""} {
		_, err := e.svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: pin})
		if !errors.Is(err, auth.ErrPINIncorrect) {
			t.Fatalf("PIN %q: want ErrPINIncorrect, got %v", pin, err)
		}
	}
	e.drain(t)
	if n := e.instanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow started after wrong PINs: %d instances", n)
	}
	var attempts int
	_ = e.pool.QueryRow(ctx, `SELECT pin_failed_attempts FROM users WHERE id=$1`, sender).Scan(&attempts)
	if attempts != 2 {
		t.Errorf("failed attempts = %d, want 2", attempts)
	}

	out, err := e.svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testPIN})
	if err != nil {
		t.Fatalf("correct PIN: %v", err)
	}
	e.drain(t)
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("after confirm: status %q, want completed (response status %q)", got, out.Status)
	}
	if got := testutil.WalletBalance(t, e.pool, rw); got != 100_000 {
		t.Errorf("receiver balance = %d, want 100000", got)
	}
	if got := testutil.WalletBalance(t, e.pool, sw); got != 2_000_000-100_000-res.Fee {
		t.Errorf("sender balance = %d, want %d", got, 2_000_000-100_000-res.Fee)
	}

	// TC-AUT-013: confirming again returns the state and does not start a second workflow.
	if _, err := e.svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testPIN}); err != nil {
		t.Fatalf("second confirm: %v", err)
	}
	e.drain(t)
	if n := e.instanceCount(t, res.TransactionID); n != 1 {
		t.Fatalf("workflow instances after a second confirm = %d, want 1", n)
	}
	if got := testutil.WalletBalance(t, e.pool, rw); got != 100_000 {
		t.Errorf("receiver balance after second confirm = %d, want 100000", got)
	}
}

// TC-AUT-005: concurrent confirms start the workflow once and move money once.
func TestConfirm_ConcurrentStartsWorkflowOnce(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	sender, _, _ := e.userWithMoney(t, 2_000_000)
	e.setPIN(t, sender, testPIN)
	_, rw, rcvPhone := e.userWithMoney(t, 0)
	res := e.requestP2P(t, sender, rcvPhone, 100_000)

	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = e.svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testPIN})
		}(i)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("confirm %d: %v", i, err)
		}
	}
	e.drain(t)
	if n := e.instanceCount(t, res.TransactionID); n != 1 {
		t.Fatalf("workflow instances = %d, want exactly 1", n)
	}
	if got := testutil.WalletBalance(t, e.pool, rw); got != 100_000 {
		t.Errorf("receiver balance = %d, want 100000 (moved once)", got)
	}
}

// TC-AUT-006: a user without a PIN cannot confirm a pin transaction.
func TestConfirm_NoPinSet(t *testing.T) {
	e := newEnv(t, "")
	sender, _, _ := e.userWithMoney(t, 2_000_000)
	_, _, rcvPhone := e.userWithMoney(t, 0)
	res := e.requestP2P(t, sender, rcvPhone, 100_000)
	_, err := e.svc.ExecuteEngineConfirm(context.Background(), transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testPIN})
	if !errors.Is(err, auth.ErrPINNotSet) {
		t.Fatalf("want ErrPINNotSet, got %v", err)
	}
	if n := e.instanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow started without a PIN: %d", n)
	}
}

// AC-4 through the confirm path: five wrong PINs lock further attempts, even a correct one.
func TestConfirm_LockoutRefusesCorrectPin(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	sender, _, _ := e.userWithMoney(t, 2_000_000)
	e.setPIN(t, sender, testPIN)
	_, _, rcvPhone := e.userWithMoney(t, 0)
	res := e.requestP2P(t, sender, rcvPhone, 100_000)
	for i := 0; i < 5; i++ {
		_, _ = e.svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: "908172"})
	}
	_, err := e.svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testPIN})
	if !errors.Is(err, auth.ErrPINLocked) {
		t.Fatalf("correct PIN during the lock: want ErrPINLocked, got %v", err)
	}
	if n := e.instanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow started during a lock: %d", n)
	}
}

// TC-AUT-007: the 2fa band has no provider, so it is refused (fail closed).
func TestRequest_TwoFactorBandIsRefused(t *testing.T) {
	e := newEnv(t, "")
	// The 2fa band starts at 50,000,000. Fund through the ledger path in two steps.
	sender, sw, _ := e.userWithMoney(t, 0)
	e.setOffer(t, sender, "OFFER_CUST_L2") // 60M is above the 2fa threshold and under the L2 cap
	for i := 0; i < 2; i++ {
		fund(t, e.pool, sender, sw, 40_000_000)
	}
	_, _, rcvPhone := e.userWithMoney(t, 0)
	// funding posts p2p rows initiated by the sender, which the interval throttle would count
	if _, err := e.pool.Exec(context.Background(), `UPDATE transactions SET created_at = created_at - interval '1 hour' WHERE initiated_by = $1`, sender); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.ExecuteEngineRequest(context.Background(), transfer.EngineRequestInput{
		ServiceCode: "P2P_TRANSFER", SenderUserID: sender, ReceiverPhone: rcvPhone,
		Amount: 60_000_000, Idempotency: wallet.NewRefCode(),
	})
	if !errors.Is(err, transfer.ErrAuthMethodUnavailable) {
		t.Fatalf("a request that needs 2fa must be refused with ErrAuthMethodUnavailable, got %v", err)
	}
}

// TC-AUT-008: an unconfirmed request expires without moving money.
func TestAwaitingAuth_ExpiresWithoutMovingMoney(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	sender, sw, _ := e.userWithMoney(t, 2_000_000)
	_, _, rcvPhone := e.userWithMoney(t, 0)
	res := e.requestP2P(t, sender, rcvPhone, 100_000)
	if _, err := e.pool.Exec(ctx, `UPDATE transactions SET created_at = now() - interval '11 minutes' WHERE id=$1`, res.TransactionID); err != nil {
		t.Fatal(err)
	}
	n, err := e.svc.ExpireAwaitingAuth(ctx, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expired %d, want at least 1", n)
	}
	if got := e.txnStatus(t, res.RefCode); got != "failed" {
		t.Fatalf("status = %q, want failed", got)
	}
	var reason string
	_ = e.pool.QueryRow(ctx, `SELECT coalesce(failure_reason,'') FROM transactions WHERE id=$1`, res.TransactionID).Scan(&reason)
	if reason != "auth_expired" {
		t.Errorf("failure_reason = %q, want auth_expired", reason)
	}
	if got := testutil.WalletBalance(t, e.pool, sw); got != 2_000_000 {
		t.Errorf("sender balance changed to %d", got)
	}
	// a confirm after expiry must not resurrect it
	e.setPIN(t, sender, testPIN)
	if _, err := e.svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testPIN}); err == nil {
		t.Fatal("confirming an expired request must fail")
	}
	if n := e.instanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("workflow started for an expired request: %d", n)
	}
}

// TC-AUT-011: unconfirmed requests do not consume the daily quota.
func TestOfferGuard_AwaitingAuthDoesNotConsumeQuota(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	sender, _, _ := e.userWithMoney(t, 1_000_000)
	_, _, rcvPhone := e.userWithMoney(t, 0)
	e.setPIN(t, sender, testPIN)
	for i := 0; i < 3; i++ {
		_ = e.requestP2P(t, sender, rcvPhone, 100_000)
		// the interval throttle would reject an immediate second attempt; age the previous row
		_, _ = e.pool.Exec(ctx, `UPDATE transactions SET created_at = created_at - interval '1 minute' WHERE initiated_by=$1`, sender)
	}
	var sum int64
	if err := e.pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount + fee), 0)::bigint FROM transactions WHERE initiated_by=$1 AND type='p2p' AND status NOT IN ('failed','reversed','awaiting_auth')`, sender).Scan(&sum); err != nil {
		t.Fatal(err)
	}
	if sum != 0 {
		t.Fatalf("quota sum with only awaiting_auth requests = %d, want 0", sum)
	}
}

// confirmWithPIN gives the sender a PIN (once) and confirms a held request, the
// way a client does. Tests that drive money through ExecuteEngineRequest use it
// instead of relying on the request starting the workflow by itself.
func (e *env) confirmWithPIN(t *testing.T, sender uuid.UUID, res transfer.EngineRequestResult) {
	t.Helper()
	if res.Status != "awaiting_auth" {
		return // an offer that needs no confirmation already started
	}
	var has bool
	if err := e.pool.QueryRow(context.Background(), `SELECT pin_hash IS NOT NULL FROM users WHERE id=$1`, sender).Scan(&has); err != nil {
		t.Fatal(err)
	}
	if !has {
		e.setPIN(t, sender, testPIN)
	}
	if _, err := e.svc.ExecuteEngineConfirm(context.Background(), transfer.EngineConfirmInput{
		TransactionID: res.TransactionID, SenderUserID: sender, PIN: testPIN,
	}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
}
