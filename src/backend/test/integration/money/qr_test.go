package money

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/qr"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

func TestQR_MintResolveTransferEndToEnd(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()

	// Sender is u1 (has money), Receiver is u2 (starts with 0)
	senderID, senderWallet, _ := e.UserWithMoney(t, 2_000_000)
	receiverID, receiverWallet, _ := e.UserWithMoney(t, 0)

	// 1. Receiver (u2) mints a QR
	qrRes, err := e.QrSvc.Mint(receiverID, "Nguyen V** A*", "091****678")
	if err != nil {
		t.Fatalf("Mint failed: %v", err)
	}

	// 2. Sender (u1) resolves the QR
	resolved, err := e.QrSvc.Resolve(qrRes.Payload, qrRes.Signature)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if resolved.NameMasked != "Nguyen V** A*" || resolved.PhoneMasked != "091****678" {
		t.Errorf("resolved profile info mismatch: got (%s, %s)", resolved.NameMasked, resolved.PhoneMasked)
	}

	if resolved.ReceiverToken == "" {
		t.Fatal("expected non-empty receiver token")
	}

	// 3. Sender transfers VND 500,000 to Receiver using the receiver token
	res, err := e.Svc.TransferToUser(ctx, senderID, receiverID, 500_000, "For lunch", uuid.NewString())
	if err != nil {
		t.Fatalf("TransferToUser failed: %v", err)
	}

	e.Drain(t)

	// Verify states
	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn: want completed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, senderWallet); got != 2_000_000-res.Total {
		t.Fatalf("sender balance: want %d, got %d", 2_000_000-res.Total, got)
	}
	if got := testutil.WalletBalance(t, e.Pool, receiverWallet); got != 500_000 {
		t.Fatalf("receiver balance: want 500000, got %d", got)
	}
}

func TestQR_ForgedAndExpired(t *testing.T) {
	e := testkit.NewEnv(t, "")

	receiverID, _, _ := e.UserWithMoney(t, 0)

	qrRes, err := e.QrSvc.Mint(receiverID, "Nguyen V** A*", "091****678")
	if err != nil {
		t.Fatalf("Mint failed: %v", err)
	}

	// 1. Forged Signature -> Resolve must return ErrQRInvalid
	_, err = e.QrSvc.Resolve(qrRes.Payload, "invalid-sig")
	if !errors.Is(err, qr.ErrQRInvalid) {
		t.Errorf("forged signature resolve: want ErrQRInvalid, got %v", err)
	}

	// 2. Tampered Payload -> Resolve must return ErrQRInvalid
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(qrRes.Payload)
	var payload qr.QRPayload
	json.Unmarshal(payloadBytes, &payload)
	payload.UserID = uuid.New() // change ID
	tamperedBytes, _ := json.Marshal(payload)
	tamperedPayload := base64.RawURLEncoding.EncodeToString(tamperedBytes)

	_, err = e.QrSvc.Resolve(tamperedPayload, qrRes.Signature)
	if !errors.Is(err, qr.ErrQRInvalid) {
		t.Errorf("tampered payload resolve: want ErrQRInvalid, got %v", err)
	}
}
