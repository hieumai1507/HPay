package testkit

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

const ValidPIN = "482915"

// setPIN gives a user a real PIN through the same code path the API uses.
func (e *Env) SetPIN(t *testing.T, userID uuid.UUID, pin string) {
	t.Helper()
	pw := "S3cret!password"
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pool.Exec(context.Background(), `UPDATE users SET password_hash=$2 WHERE id=$1`, userID, h); err != nil {
		t.Fatal(err)
	}
	if err := e.Auth.SetPIN(context.Background(), userID, pw, pin); err != nil {
		t.Fatalf("set PIN: %v", err)
	}
}

func (e *Env) RequestP2P(t *testing.T, sender uuid.UUID, receiverPhone string, amount int64) transfer.EngineRequestResult {
	t.Helper()
	res, err := e.Svc.ExecuteEngineRequest(context.Background(), transfer.EngineRequestInput{
		ServiceCode: "P2P_TRANSFER", SenderUserID: sender, ReceiverPhone: receiverPhone,
		Amount: amount, Idempotency: wallet.NewRefCode(),
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return res
}

func (e *Env) InstanceCount(t *testing.T, txnID uuid.UUID) int {
	t.Helper()
	var n int
	if err := e.Pool.QueryRow(context.Background(), `SELECT count(*) FROM workflow_instances WHERE transaction_id=$1`, txnID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// confirmWithPIN gives the sender a PIN (once) and confirms a held request, the
// way a client does. Tests that drive money through ExecuteEngineRequest use it
// instead of relying on the request starting the workflow by itself.
func (e *Env) ConfirmWithPIN(t *testing.T, sender uuid.UUID, res transfer.EngineRequestResult) {
	t.Helper()
	if res.Status != "awaiting_auth" {
		return // an offer that needs no confirmation already started
	}
	var has bool
	if err := e.Pool.QueryRow(context.Background(), `SELECT pin_hash IS NOT NULL FROM users WHERE id=$1`, sender).Scan(&has); err != nil {
		t.Fatal(err)
	}
	if !has {
		e.SetPIN(t, sender, ValidPIN)
	}
	if _, err := e.Svc.ExecuteEngineConfirm(context.Background(), transfer.EngineConfirmInput{
		TransactionID: res.TransactionID, SenderUserID: sender, PIN: ValidPIN,
	}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
}
