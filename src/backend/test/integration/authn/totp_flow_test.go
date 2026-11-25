package authn

// ADR-0017: the 2fa band through the real service. Cases TC-2FA-010..015.
// A transaction of $2,400.00 (240,000 cents) is in the 2fa band (above 200,000 cents).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

const bigAmount = 240_000

// twoFactorSender funds a user (L2 offer, PIN set), returns the user, receiver phone and receiver wallet.
func twoFactorSender(t *testing.T, e *testkit.Env) (uuid.UUID, string, uuid.UUID) {
	t.Helper()
	sender, sw, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, sender, "OFFER_CUST_L2")
	for i := 0; i < 2; i++ {
		testkit.Fund(t, e.Pool, sender, sw, 160_000)
	}
	e.SetPIN(t, sender, testkit.ValidPIN)
	_, rw, rcv := e.UserWithMoney(t, 0)
	testkit.AgeUserTransactions(t, e, sender, "1 hour")
	return sender, rcv, rw
}

// enrolTOTP enrols and activates TOTP for the user and returns the secret and recovery codes.
func enrolTOTP(t *testing.T, e *testkit.Env, user uuid.UUID) ([]byte, []string) {
	t.Helper()
	ctx := context.Background()
	pw := "S3cret!password"
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, user, h); err != nil {
		t.Fatal(err)
	}
	out, err := e.Auth.EnrollTOTP(ctx, user, pw)
	if err != nil {
		t.Fatalf("enrol: %v", err)
	}
	secret, err := auth.DecodeTOTPSecret(out.Secret)
	if err != nil {
		t.Fatal(err)
	}
	conf, err := e.Auth.ConfirmTOTP(ctx, user, auth.TOTPCode(secret, time.Now().Unix()/30))
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	return secret, conf.RecoveryCodes
}

// freshCode is a valid code for a time step later than any used so far.
func freshCode(secret []byte, ahead int64) string {
	return auth.TOTPCode(secret, time.Now().Unix()/30+ahead)
}

func request2FA(e *testkit.Env, sender uuid.UUID, rcv string) (transfer.EngineRequestResult, error) {
	return e.Svc.ExecuteEngineRequest(context.Background(), transfer.EngineRequestInput{
		ServiceCode: "P2P_TRANSFER", SenderUserID: sender, ReceiverPhone: rcv, Amount: bigAmount, Idempotency: uuid.NewString(),
	})
}

// TC-2FA-010
func TestRequest_TwoFactorBandIsHeldWhenEnrolled(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, rcv, _ := twoFactorSender(t, e)
	enrolTOTP(t, e, sender)
	res, err := request2FA(e, sender, rcv)
	if err != nil {
		t.Fatalf("an enrolled user must be able to request a 2fa transaction: %v", err)
	}
	if res.AuthRequired != "2fa" || res.Status != "awaiting_auth" {
		t.Fatalf("want auth_required 2fa and awaiting_auth, got %q / %q", res.AuthRequired, res.Status)
	}
	if got := e.TxnStatus(t, res.RefCode); got != "awaiting_auth" {
		t.Fatalf("stored status %q", got)
	}
}

// TC-2FA-011
func TestConfirm_TwoFactorStartsWorkflowWithPINAndTOTP(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, rcv, rw := twoFactorSender(t, e)
	secret, _ := enrolTOTP(t, e, sender)
	res, err := request2FA(e, sender, rcv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Svc.ExecuteEngineConfirm(context.Background(), transfer.EngineConfirmInput{
		TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN, TOTP: freshCode(secret, 1),
	}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	e.Drain(t)
	if n := e.InstanceCount(t, res.TransactionID); n != 1 {
		t.Fatalf("workflow instances = %d, want 1", n)
	}
	if got := testutil.WalletBalance(t, e.Pool, rw); got != bigAmount {
		t.Fatalf("receiver balance = %d, want %d", got, bigAmount)
	}
}

// TC-2FA-012
func TestConfirm_TwoFactorWithoutValidTOTPDoesNotStart(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, rcv, _ := twoFactorSender(t, e)
	secret, _ := enrolTOTP(t, e, sender)
	res, err := request2FA(e, sender, rcv)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN}); !errors.Is(err, transfer.ErrCredentialRequired) {
		t.Fatalf("missing TOTP: want ErrCredentialRequired, got %v", err)
	}
	if _, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN, TOTP: "000000"}); !errors.Is(err, auth.ErrTOTPInvalid) {
		t.Fatalf("wrong TOTP: want ErrTOTPInvalid, got %v", err)
	}
	if n := e.InstanceCount(t, res.TransactionID); n != 0 {
		t.Fatalf("no workflow may start without a valid TOTP, got %d", n)
	}
	// five wrong codes in a row lock TOTP use; then even the right code is refused
	for i := 0; i < 4; i++ {
		_, _ = e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN, TOTP: "000000"})
	}
	if _, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN, TOTP: freshCode(secret, 1)}); !errors.Is(err, auth.ErrTOTPLocked) {
		t.Fatalf("after the lockout even the right code is refused, got %v", err)
	}
}

// TC-2FA-013
func TestRequest_TwoFactorBandRefusedWithoutEnrolment(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, rcv, _ := twoFactorSender(t, e)
	_, err := request2FA(e, sender, rcv)
	if !errors.Is(err, auth.ErrTOTPNotEnrolled) {
		t.Fatalf("want ErrTOTPNotEnrolled, got %v", err)
	}
	var n int
	if err := e.Pool.QueryRow(context.Background(), `SELECT count(*) FROM transactions WHERE initiated_by=$1 AND status='awaiting_auth'`, sender).Scan(&n); err != nil || n != 0 {
		t.Fatalf("nothing may be held for an unenrolled user (held=%d err=%v)", n, err)
	}
}

// TC-2FA-014
func TestConfirm_PINBandIgnoresTOTP(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, sw, _ := e.UserWithMoney(t, 8_000)
	_ = sw
	e.SetPIN(t, sender, testkit.ValidPIN)
	_, _, rcv := e.UserWithMoney(t, 0)
	res := e.RequestP2P(t, sender, rcv, 400)
	if res.AuthRequired != "pin" {
		t.Fatalf("setup: want the pin band, got %q", res.AuthRequired)
	}
	if _, err := e.Svc.ExecuteEngineConfirm(context.Background(), transfer.EngineConfirmInput{
		TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN, TOTP: "999999",
	}); err != nil {
		t.Fatalf("a TOTP field on a PIN-band transaction must be ignored: %v", err)
	}
}

// TC-2FA-015
func TestConfirm_TwoFactorAcceptsRecoveryCodeOnce(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, rcv, _ := twoFactorSender(t, e)
	_, codes := enrolTOTP(t, e, sender)
	ctx := context.Background()
	first, err := request2FA(e, sender, rcv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: first.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN, TOTP: codes[0]}); err != nil {
		t.Fatalf("a recovery code must be accepted in place of the TOTP: %v", err)
	}
	e.Drain(t)
	// a second large transaction (L2 cap allows it after more funding)
	testkit.Fund(t, e.Pool, sender, mainWallet(t, e, sender), 280_000)
	testkit.AgeUserTransactions(t, e, sender, "1 hour")
	second, err := request2FA(e, sender, rcv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{TransactionID: second.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN, TOTP: codes[0]}); !errors.Is(err, auth.ErrTOTPInvalid) {
		t.Fatalf("a used recovery code must be refused, got %v", err)
	}
}

func mainWallet(t *testing.T, e *testkit.Env, user uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.Pool.QueryRow(context.Background(), `SELECT id FROM wallets WHERE owner_id=$1 AND purpose='main'`, user).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
