package money

import (
	"context"
	"testing"

	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// AC-1: a new user's wallet is USD.
func TestCurrency_NewUserWalletIsUSD(t *testing.T) {
	e := testkit.NewEnv(t, "")
	userID, walletID, _ := e.UserWithMoney(t, 0)
	if err := e.Wsvc.EnsureMainWallet(context.Background(), userID); err != nil {
		t.Fatalf("ensure wallet: %v", err)
	}
	var cur string
	if err := e.Pool.QueryRow(context.Background(), `SELECT currency FROM wallets WHERE id=$1`, walletID).Scan(&cur); err != nil {
		t.Fatal(err)
	}
	if cur != wallet.DefaultCurrency {
		t.Fatalf("main wallet currency = %q, want %q", cur, wallet.DefaultCurrency)
	}
}

// AC-1: the three system wallets are USD.
func TestCurrency_SystemWalletsAreUSD(t *testing.T) {
	e := testkit.NewEnv(t, "")
	rows, err := e.Pool.Query(context.Background(),
		`SELECT system_code, currency FROM wallets WHERE system_code IN ('SYSTEM_CASH_IN','SYSTEM_FEE','SYSTEM_SUSPENSE')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var code, cur string
		if err := rows.Scan(&code, &cur); err != nil {
			t.Fatal(err)
		}
		n++
		if cur != "USD" {
			t.Errorf("%s currency = %q, want USD", code, cur)
		}
	}
	if n != 3 {
		t.Fatalf("want 3 system wallets, found %d", n)
	}
}

// AC-1: the database default is USD.
func TestCurrency_DatabaseDefaultIsUSD(t *testing.T) {
	e := testkit.NewEnv(t, "")
	var def string
	if err := e.Pool.QueryRow(context.Background(), `
		SELECT column_default FROM information_schema.columns
		WHERE table_name='wallets' AND column_name='currency'`).Scan(&def); err != nil {
		t.Fatal(err)
	}
	if def != "'USD'::character varying" {
		t.Fatalf("wallets.currency default = %s, want 'USD'", def)
	}
}
