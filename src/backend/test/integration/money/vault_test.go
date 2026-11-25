package money

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/ledger"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/vault"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// vaultMove drives a deposit/withdraw through the same 3-step engine the mobile
// app uses — there is deliberately no vault-specific money endpoint.
func vaultMove(t *testing.T, e *testkit.Env, userID, vaultID uuid.UUID, amount int64, direction string) (transfer.EngineRequestResult, error) {
	t.Helper()
	res, err := e.Svc.ExecuteEngineRequest(context.Background(), transfer.EngineRequestInput{
		ServiceCode:  "VAULT_SAVINGS",
		SenderUserID: userID,
		Amount:       amount,
		Idempotency:  uuid.NewString(),
		ExtraParams: map[string]interface{}{
			"vault_id":  vaultID.String(),
			"direction": direction,
		},
	})
	if err == nil {
		e.ConfirmWithPIN(t, userID, res) // a real client confirms with the PIN
	}
	return res, err
}

func newVault(t *testing.T, e *testkit.Env, userID uuid.UUID, label string) uuid.UUID {
	t.Helper()
	v, err := vault.New(e.Pool).Create(context.Background(), userID, label, 0)
	if err != nil {
		t.Fatalf("create vault %q: %v", label, err)
	}
	return v.ID
}

// Money into a vault is a real ledger movement between two wallets of the same
// user: main goes down, the pocket goes up, and nothing is created or lost.
func TestVault_DepositMovesMoneyAndBalances(t *testing.T) {
	e := testkit.NewEnv(t, "")
	userID, mainWallet, _ := e.UserWithMoney(t, 0)
	testkit.Fund(t, e.Pool, userID, mainWallet, 8_000)
	vaultID := newVault(t, e, userID, "Holiday fund")

	mainBefore := testutil.WalletBalance(t, e.Pool, mainWallet)

	res, err := vaultMove(t, e, userID, vaultID, 2_000, "deposit")
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}
	e.Drain(t)
	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("deposit status: want completed, got %s", got)
	}

	if got := testutil.WalletBalance(t, e.Pool, mainWallet); got != mainBefore-2_000 {
		t.Fatalf("main: want %d, got %d", mainBefore-2_000, got)
	}
	if got := testutil.WalletBalance(t, e.Pool, vaultID); got != 2_000 {
		t.Fatalf("vault: want 2000, got %d", got)
	}
	// The invariant that matters: both wallets are backed by their ledger.
	if got := testutil.LedgerSum(t, e.Pool, vaultID); got != 2_000 {
		t.Fatalf("vault ledger sum: want 2000, got %d", got)
	}
}

func TestVault_WithdrawReturnsToMain(t *testing.T) {
	e := testkit.NewEnv(t, "")
	userID, mainWallet, _ := e.UserWithMoney(t, 0)
	testkit.Fund(t, e.Pool, userID, mainWallet, 8_000)
	vaultID := newVault(t, e, userID, "Emergency fund")

	dep, err := vaultMove(t, e, userID, vaultID, 3_200, "deposit")
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}
	e.Drain(t)
	if got := e.TxnStatus(t, dep.RefCode); got != "completed" {
		t.Fatalf("deposit status: %s", got)
	}
	mainAfterDeposit := testutil.WalletBalance(t, e.Pool, mainWallet)

	wd, err := vaultMove(t, e, userID, vaultID, 1_200, "withdraw")
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	e.Drain(t)
	if got := e.TxnStatus(t, wd.RefCode); got != "completed" {
		t.Fatalf("withdraw status: %s", got)
	}

	if got := testutil.WalletBalance(t, e.Pool, vaultID); got != 2_000 {
		t.Fatalf("vault after withdraw: want 2000, got %d", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, mainWallet); got != mainAfterDeposit+1_200 {
		t.Fatalf("main after withdraw: want %d, got %d", mainAfterDeposit+1_200, got)
	}
}

// A pocket cannot lend money it does not hold.
func TestVault_WithdrawOverBalanceRejected(t *testing.T) {
	e := testkit.NewEnv(t, "")
	userID, mainWallet, _ := e.UserWithMoney(t, 0)
	testkit.Fund(t, e.Pool, userID, mainWallet, 4_000)
	vaultID := newVault(t, e, userID, "Empty vault")

	_, err := vaultMove(t, e, userID, vaultID, 400, "withdraw")
	if !errors.Is(err, ledger.ErrInsufficientBalance) {
		t.Fatalf("withdraw from empty vault: err = %v, want ErrInsufficientBalance", err)
	}
	if got := testutil.WalletBalance(t, e.Pool, vaultID); got != 0 {
		t.Fatalf("vault must stay at 0, got %d", got)
	}
}

// The vault id is attacker-controlled input, so ownership is resolved in SQL.
// Somebody else's pocket must look like it does not exist at all.
func TestVault_ForeignVaultRejected(t *testing.T) {
	e := testkit.NewEnv(t, "")
	attacker, attackerWallet, _ := e.UserWithMoney(t, 0)
	testkit.Fund(t, e.Pool, attacker, attackerWallet, 4_000)
	victim, _, _ := e.UserWithMoney(t, 0)
	victimVault := newVault(t, e, victim, "Someone else's vault")

	_, err := vaultMove(t, e, attacker, victimVault, 400, "deposit")
	if !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("foreign vault: err = %v, want vault.ErrNotFound", err)
	}
}

func TestVault_DirectionMustBeValid(t *testing.T) {
	e := testkit.NewEnv(t, "")
	userID, mainWallet, _ := e.UserWithMoney(t, 0)
	testkit.Fund(t, e.Pool, userID, mainWallet, 4_000)
	vaultID := newVault(t, e, userID, "Direction test vault")

	if _, err := vaultMove(t, e, userID, vaultID, 400, "sideways"); !errors.Is(err, vault.ErrDirectionInvalid) {
		t.Fatalf("bad direction: err = %v, want ErrDirectionInvalid", err)
	}
}

// Migration 000008 widened wallet_owner_unique to include label; before it, a
// user was capped at a single vault.
func TestVault_MultipleVaultsPerUser(t *testing.T) {
	e := testkit.NewEnv(t, "")
	userID, _, _ := e.UserWithMoney(t, 0)

	svc := vault.New(e.Pool)
	if _, err := svc.Create(context.Background(), userID, "Vault A", 4_000); err != nil {
		t.Fatalf("first vault: %v", err)
	}
	if _, err := svc.Create(context.Background(), userID, "Vault B", 0); err != nil {
		t.Fatalf("second vault must be allowed: %v", err)
	}
	if _, err := svc.Create(context.Background(), userID, "Vault A", 0); !errors.Is(err, vault.ErrLabelTaken) {
		t.Fatalf("duplicate label: err = %v, want ErrLabelTaken", err)
	}

	list, err := svc.List(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 vaults, got %d", len(list))
	}
}
