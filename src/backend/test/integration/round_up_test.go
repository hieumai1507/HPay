package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/vault"
)

// Round-up (docs plan round-up-vault): the optional ROUND_UP step appended
// after CONFIRM in p2p_transfer v2 sweeps a P2P spend's rounding difference
// into the sender's designated vault. Every "cannot apply" branch must be a
// no-op, never a failed transaction — CONFIRM has already completed by then.

func setRoundUpTarget(t *testing.T, e *env, userID, vaultID uuid.UUID, enabled bool) vault.Vault {
	t.Helper()
	v, err := vault.New(e.pool).SetRoundUpTarget(context.Background(), userID, vaultID, enabled)
	if err != nil {
		t.Fatalf("SetRoundUpTarget: %v", err)
	}
	return v
}

func TestRoundUp_AppliesDifference(t *testing.T) {
	e := newEnv(t, "")
	userID, senderWallet, _ := e.userWithMoney(t, 2_000_000)
	_, _, receiverPhone := e.userWithMoney(t, 0)
	vaultID := newVault(t, e, userID, "Làm tròn")
	setRoundUpTarget(t, e, userID, vaultID, true)

	senderBefore := testutil.WalletBalance(t, e.pool, senderWallet)

	res, err := e.svc.Transfer(context.Background(), userID, receiverPhone, 123_000, "coffee", uuid.NewString())
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	e.drain(t)
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn status: want completed, got %s", got)
	}

	const wantDiff = 7_000 // 123_000 rounds up to 130_000
	if got := testutil.WalletBalance(t, e.pool, vaultID); got != wantDiff {
		t.Fatalf("vault balance: want %d, got %d", wantDiff, got)
	}
	if got := testutil.LedgerSum(t, e.pool, vaultID); got != wantDiff {
		t.Fatalf("vault ledger sum: want %d, got %d", wantDiff, got)
	}
	wantSender := senderBefore - 123_000 - res.Fee - wantDiff
	if got := testutil.WalletBalance(t, e.pool, senderWallet); got != wantSender {
		t.Fatalf("sender balance: want %d, got %d", wantSender, got)
	}
}

func TestRoundUp_ExactAmount_Skipped(t *testing.T) {
	e := newEnv(t, "")
	userID, _, _ := e.userWithMoney(t, 2_000_000)
	_, _, receiverPhone := e.userWithMoney(t, 0)
	vaultID := newVault(t, e, userID, "Làm tròn")
	setRoundUpTarget(t, e, userID, vaultID, true)

	res, err := e.svc.Transfer(context.Background(), userID, receiverPhone, 120_000, "exact", uuid.NewString())
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	e.drain(t)
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn status: want completed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.pool, vaultID); got != 0 {
		t.Fatalf("vault must stay empty on an exact multiple, got %d", got)
	}
}

func TestRoundUp_NoTarget_Skipped(t *testing.T) {
	e := newEnv(t, "")
	userID, senderWallet, _ := e.userWithMoney(t, 2_000_000)
	_, _, receiverPhone := e.userWithMoney(t, 0)
	senderBefore := testutil.WalletBalance(t, e.pool, senderWallet)

	res, err := e.svc.Transfer(context.Background(), userID, receiverPhone, 123_000, "no target", uuid.NewString())
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	e.drain(t)
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn status: want completed, got %s", got)
	}
	wantSender := senderBefore - 123_000 - res.Fee
	if got := testutil.WalletBalance(t, e.pool, senderWallet); got != wantSender {
		t.Fatalf("sender balance: want %d, got %d (round-up must not fire without a target)", wantSender, got)
	}
}

func TestRoundUp_InsufficientBalance_SkippedNotFailed(t *testing.T) {
	e := newEnv(t, "")
	userID, senderWallet, _ := e.userWithMoney(t, 0)
	_, _, receiverPhone := e.userWithMoney(t, 0)
	vaultID := newVault(t, e, userID, "Làm tròn")
	setRoundUpTarget(t, e, userID, vaultID, true)

	// Fund exactly amount+fee so nothing is left over for the round-up leg.
	_, total, err := e.svc.FeePreview(context.Background(), userID, 123_000)
	if err != nil {
		t.Fatalf("fee preview: %v", err)
	}
	fund(t, e.pool, userID, senderWallet, total)

	res, err := e.svc.Transfer(context.Background(), userID, receiverPhone, 123_000, "tight", uuid.NewString())
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	e.drain(t)
	// The main transfer must complete regardless of round-up's outcome.
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn status: want completed even when round-up can't apply, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.pool, senderWallet); got != 0 {
		t.Fatalf("sender: want 0 left (no round-up leg), got %d", got)
	}
	if got := testutil.WalletBalance(t, e.pool, vaultID); got != 0 {
		t.Fatalf("vault must stay empty when sender can't afford the round-up, got %d", got)
	}
}

func TestRoundUp_ChangingTarget_UnsetsOld(t *testing.T) {
	e := newEnv(t, "")
	userID, _, _ := e.userWithMoney(t, 2_000_000)
	_, _, receiverPhone := e.userWithMoney(t, 0)
	vaultA := newVault(t, e, userID, "Hũ A")
	vaultB := newVault(t, e, userID, "Hũ B")

	setRoundUpTarget(t, e, userID, vaultA, true)
	setRoundUpTarget(t, e, userID, vaultB, true)

	svc := vault.New(e.pool)
	list, err := svc.List(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	targets := map[uuid.UUID]bool{}
	for _, v := range list {
		targets[v.ID] = v.RoundUpTarget
	}
	if targets[vaultA] {
		t.Fatalf("vault A must have been auto-unset when B was enabled")
	}
	if !targets[vaultB] {
		t.Fatalf("vault B must be the active round-up target")
	}

	res, err := e.svc.Transfer(context.Background(), userID, receiverPhone, 123_000, "retarget", uuid.NewString())
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	e.drain(t)
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn status: want completed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.pool, vaultA); got != 0 {
		t.Fatalf("vault A: want 0, got %d", got)
	}
	if got := testutil.WalletBalance(t, e.pool, vaultB); got != 7_000 {
		t.Fatalf("vault B: want 7000, got %d", got)
	}
}

func TestRoundUp_TargetIsOwnerScoped(t *testing.T) {
	e := newEnv(t, "")
	attacker, _, _ := e.userWithMoney(t, 0)
	victim, _, _ := e.userWithMoney(t, 0)
	victimVault := newVault(t, e, victim, "Không phải của bạn")

	if _, err := vault.New(e.pool).SetRoundUpTarget(context.Background(), attacker, victimVault, true); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("foreign vault: err = %v, want vault.ErrNotFound", err)
	}
}
