package money

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/hieumai1507/hpay-backend/internal/api"
	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TC-IDM-001: same user, same key, same payload -> the original result, one transaction.
func TestIdempotency_Transfer_ReplaySameUserSamePayload(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.UserWithMoney(t, 2_000_000)
	receiverID, _, _ := e.UserWithMoney(t, 0)
	key := uuid.NewString()

	first, err := e.Svc.TransferToUser(ctx, senderID, receiverID, 100_000, "lunch", key)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := e.Svc.TransferToUser(ctx, senderID, receiverID, 100_000, "lunch", key)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !second.Duplicate || second.RefCode != first.RefCode {
		t.Fatalf("replay must return the original: first=%+v second=%+v", first, second)
	}
	if n := e.CountTxnsByKey(t, senderID, key); n != 1 {
		t.Fatalf("want exactly 1 transaction for the key, got %d", n)
	}
	e.Drain(t)
	// however far the workflow got (it may wait for auth), money moved at most once
	if net := e.TxnLedgerNet(t, first.RefCode, senderWallet); net != 0 && net != -first.Total {
		t.Fatalf("sender ledger net for the transaction must be 0 or -%d, got %d", first.Total, net)
	}
}

// TC-IDM-002: same user, same key, different payload -> ERR_IDEMPOTENCY_KEY_REUSED.
func TestIdempotency_Transfer_SameKeyDifferentPayloadRejected(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, _, _ := e.UserWithMoney(t, 2_000_000)
	receiverID, _, _ := e.UserWithMoney(t, 0)
	otherID, _, _ := e.UserWithMoney(t, 0)
	key := uuid.NewString()

	if _, err := e.Svc.TransferToUser(ctx, senderID, receiverID, 100_000, "a", key); err != nil {
		t.Fatalf("first: %v", err)
	}
	cases := map[string]func() error{
		"amount":   func() error { _, err := e.Svc.TransferToUser(ctx, senderID, receiverID, 200_000, "a", key); return err },
		"receiver": func() error { _, err := e.Svc.TransferToUser(ctx, senderID, otherID, 100_000, "a", key); return err },
		"note":     func() error { _, err := e.Svc.TransferToUser(ctx, senderID, receiverID, 100_000, "b", key); return err },
	}
	for name, call := range cases {
		if err := call(); !errors.Is(err, wallet.ErrIdempotencyKeyReused) {
			t.Fatalf("%s changed: want ErrIdempotencyKeyReused, got %v", name, err)
		}
	}
	if n := e.CountTxnsByKey(t, senderID, key); n != 1 {
		t.Fatalf("want 1 transaction, got %d", n)
	}
}

// TC-IDM-003: two users with the same key are independent; nothing leaks.
func TestIdempotency_TwoUsersSameKeyAreIndependent(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	a, aWallet, _ := e.UserWithMoney(t, 0)
	b, bWallet, _ := e.UserWithMoney(t, 0)
	key := uuid.NewString()

	ra, err := e.Wsvc.Topup(ctx, a, 300_000, key)
	if err != nil {
		t.Fatalf("A: %v", err)
	}
	rb, err := e.Wsvc.Topup(ctx, b, 700_000, key) // different payload, same key, other user
	if err != nil {
		t.Fatalf("B must not collide with A's key: %v", err)
	}
	if rb.Duplicate || rb.RefCode == ra.RefCode || rb.Amount != 700_000 {
		t.Fatalf("B received A's transaction: A=%+v B=%+v", ra, rb)
	}
	if got := testutil.WalletBalance(t, e.Pool, aWallet); got != 300_000 {
		t.Fatalf("A balance %d", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, bWallet); got != 700_000 {
		t.Fatalf("B balance %d", got)
	}
	// each user still replays their own
	ra2, err := e.Wsvc.Topup(ctx, a, 300_000, key)
	if err != nil || !ra2.Duplicate || ra2.RefCode != ra.RefCode {
		t.Fatalf("A replay: %+v %v", ra2, err)
	}
}

// TC-IDM-004: a money-moving write without a key is rejected at the service layer
// (no server-generated fallback).
func TestIdempotency_MissingKeyRejectedInServices(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, _, _ := e.UserWithMoney(t, 2_000_000)
	receiverID, _, receiverPhone := e.UserWithMoney(t, 0)

	if _, err := e.Svc.ExecuteEngineRequest(ctx, transferEngineInput(senderID, receiverPhone, "")); !errors.Is(err, wallet.ErrIdempotencyKeyRequired) {
		t.Fatalf("engine: want ErrIdempotencyKeyRequired, got %v", err)
	}
	if _, err := e.Svc.TransferToUser(ctx, senderID, receiverID, 1_000, "", ""); !errors.Is(err, wallet.ErrIdempotencyKeyRequired) {
		t.Fatalf("transfer: want ErrIdempotencyKeyRequired, got %v", err)
	}
	if _, err := e.Wsvc.Topup(ctx, senderID, 1_000, ""); !errors.Is(err, wallet.ErrIdempotencyKeyRequired) {
		t.Fatalf("topup: want ErrIdempotencyKeyRequired, got %v", err)
	}
	if _, err := e.Msvc.Create(ctx, senderID, receiverPhone, 1_000, "", ""); !errors.Is(err, wallet.ErrIdempotencyKeyRequired) {
		t.Fatalf("money request create: want ErrIdempotencyKeyRequired, got %v", err)
	}
	if _, err := e.Msvc.CreateSplit(ctx, senderID, []string{receiverPhone}, 1_000, "", nil, ""); !errors.Is(err, wallet.ErrIdempotencyKeyRequired) {
		t.Fatalf("split: want ErrIdempotencyKeyRequired, got %v", err)
	}
	if _, err := e.Msvc.Pay(ctx, senderID, uuid.New(), ""); !errors.Is(err, wallet.ErrIdempotencyKeyRequired) {
		t.Fatalf("pay: want ErrIdempotencyKeyRequired, got %v", err)
	}
}

// TC-IDM-005: concurrent requests with the same key create exactly one transaction.
func TestIdempotency_ConcurrentSameKeyOneTransaction(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.UserWithMoney(t, 1_500_000)
	receiverID, _, _ := e.UserWithMoney(t, 0)
	key := uuid.NewString()

	const n = 8
	results := make([]string, n)
	totals := make([]int64, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := e.Svc.TransferToUser(ctx, senderID, receiverID, 100_000, "race", key)
			results[i], totals[i], errs[i] = r.RefCode, r.Total, err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if results[i] != results[0] {
			t.Fatalf("all calls must return the same ref_code: %v", results)
		}
	}
	if c := e.CountTxnsByKey(t, senderID, key); c != 1 {
		t.Fatalf("want 1 transaction, got %d", c)
	}
	e.Drain(t)
	if net := e.TxnLedgerNet(t, results[0], senderWallet); net != 0 && net != -totals[0] {
		t.Fatalf("sender ledger net must be 0 or -%d (at most one debit), got %d", totals[0], net)
	}
}

func transferEngineInput(sender uuid.UUID, phone, key string) transfer.EngineRequestInput {
	return transfer.EngineRequestInput{ServiceCode: "P2P_TRANSFER", SenderUserID: sender,
		ReceiverPhone: phone, Amount: 1_000, Idempotency: key}
}

// TC-IDM-006: engine path (HTTP) - replay, 409 on a different payload, other user unaffected.
func TestIdempotency_EngineHTTP_ReplayConflictAndIsolation(t *testing.T) {
	e := testkit.NewEnv(t, "")
	a, _, _ := e.UserWithMoney(t, 1_500_000)
	b, _, _ := e.UserWithMoney(t, 1_500_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)
	key := uuid.NewString()
	body := map[string]any{"service_code": "P2P_TRANSFER", "amount": 100_000, "receiver_phone": receiverPhone}

	w1 := postTransactionRequest(transactionRequestRouter(t, e, a), body, key)
	if w1.Code != http.StatusOK {
		t.Fatalf("A first: %d %s", w1.Code, w1.Body)
	}
	var r1 struct {
		RefCode string `json:"ref_code"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &r1)

	// different payload, same user + key -> 409
	w2 := postTransactionRequest(transactionRequestRouter(t, e, a),
		map[string]any{"service_code": "P2P_TRANSFER", "amount": 200_000, "receiver_phone": receiverPhone}, key)
	if w2.Code != http.StatusConflict || !bytes.Contains(w2.Body.Bytes(), []byte("ERR_IDEMPOTENCY_KEY_REUSED")) {
		t.Fatalf("different payload: want 409 ERR_IDEMPOTENCY_KEY_REUSED, got %d %s", w2.Code, w2.Body)
	}

	// another user with the same key -> independent transaction, not A's
	wb := postTransactionRequest(transactionRequestRouter(t, e, b),
		map[string]any{"service_code": "P2P_TRANSFER", "amount": 300_000, "receiver_phone": receiverPhone}, key)
	if wb.Code != http.StatusOK {
		t.Fatalf("B with A's key: want 200, got %d %s", wb.Code, wb.Body)
	}
	var rb struct {
		RefCode string `json:"ref_code"`
		Amount  int64  `json:"amount"`
	}
	_ = json.Unmarshal(wb.Body.Bytes(), &rb)
	if rb.RefCode == r1.RefCode || rb.Amount != 300_000 {
		t.Fatalf("B got A's transaction: A=%s B=%+v", r1.RefCode, rb)
	}
}

// TC-IDM-007: money requests - create and split replay, conflict, isolation.
func TestIdempotency_MoneyRequestCreate(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	requester, _, _ := e.UserWithMoney(t, 0)
	other, _, _ := e.UserWithMoney(t, 0)
	_, _, payerPhone := e.UserWithMoney(t, 0)
	key := uuid.NewString()

	first, err := e.Msvc.Create(ctx, requester, payerPhone, 150_000, "dinner", key)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.Msvc.Create(ctx, requester, payerPhone, 150_000, "dinner", key)
	if err != nil || again.ID != first.ID {
		t.Fatalf("replay must return the same request: %+v %v", again, err)
	}
	var n int
	_ = e.Pool.QueryRow(ctx, `SELECT count(*) FROM money_requests WHERE requester_id=$1`, requester).Scan(&n)
	if n != 1 {
		t.Fatalf("want 1 money request, got %d", n)
	}
	if _, err := e.Msvc.Create(ctx, requester, payerPhone, 999_000, "dinner", key); !errors.Is(err, wallet.ErrIdempotencyKeyReused) {
		t.Fatalf("different amount: want ErrIdempotencyKeyReused, got %v", err)
	}
	// another requester, same key -> independent
	o, err := e.Msvc.Create(ctx, other, payerPhone, 150_000, "dinner", key)
	if err != nil || o.ID == first.ID {
		t.Fatalf("other user must get an independent request: %+v %v", o, err)
	}
}

func TestIdempotency_MoneyRequestSplit(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	requester, _, _ := e.UserWithMoney(t, 0)
	_, _, p1 := e.UserWithMoney(t, 0)
	_, _, p2 := e.UserWithMoney(t, 0)
	key := uuid.NewString()

	first, err := e.Msvc.CreateSplit(ctx, requester, []string{p1, p2}, 300_000, "trip", nil, key)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.Msvc.CreateSplit(ctx, requester, []string{p1, p2}, 300_000, "trip", nil, key)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if again.SplitGroupID != first.SplitGroupID || len(again.Requests) != 2 || again.Requests[0].ID != first.Requests[0].ID {
		t.Fatalf("replay must return the original split: %+v vs %+v", again, first)
	}
	var n int
	_ = e.Pool.QueryRow(ctx, `SELECT count(*) FROM money_requests WHERE requester_id=$1`, requester).Scan(&n)
	if n != 2 {
		t.Fatalf("want 2 rows, got %d", n)
	}
	if _, err := e.Msvc.CreateSplit(ctx, requester, []string{p1, p2}, 400_000, "trip", nil, key); !errors.Is(err, wallet.ErrIdempotencyKeyReused) {
		t.Fatalf("different total: want ErrIdempotencyKeyReused, got %v", err)
	}
	if _, err := e.Msvc.CreateSplit(ctx, requester, []string{p1}, 300_000, "trip", nil, key); !errors.Is(err, wallet.ErrIdempotencyKeyReused) {
		t.Fatalf("different payers: want ErrIdempotencyKeyReused, got %v", err)
	}
}

// TC-IDM-008: paying a money request - replay, key reuse for another request, isolation.
func TestIdempotency_MoneyRequestPay(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	requester, _, _ := e.UserWithMoney(t, 0)
	payer, _, payerPhone := e.UserWithMoney(t, 1_500_000)
	r1, err := e.Msvc.Create(ctx, requester, payerPhone, 100_000, "one", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	r2, err := e.Msvc.Create(ctx, requester, payerPhone, 100_000, "two", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()

	first, err := e.Msvc.Pay(ctx, payer, r1.ID, key)
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	again, err := e.Msvc.Pay(ctx, payer, r1.ID, key)
	if err != nil {
		t.Fatalf("replay pay must return the original, got %v", err)
	}
	if !again.Duplicate || again.RefCode != first.RefCode {
		t.Fatalf("replay: %+v vs %+v", again, first)
	}
	if _, err := e.Msvc.Pay(ctx, payer, r2.ID, key); !errors.Is(err, wallet.ErrIdempotencyKeyReused) {
		t.Fatalf("same key for another request: want ErrIdempotencyKeyReused, got %v", err)
	}
	if n := e.CountTxnsByKey(t, payer, key); n != 1 {
		t.Fatalf("want 1 txn, got %d", n)
	}
}

// TC-IDM-009: topup with the same key and a different amount is rejected.
func TestIdempotency_Topup_DifferentAmountRejected(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	u, w, _ := e.UserWithMoney(t, 0)
	key := uuid.NewString()
	if _, err := e.Wsvc.Topup(ctx, u, 100_000, key); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Wsvc.Topup(ctx, u, 200_000, key); !errors.Is(err, wallet.ErrIdempotencyKeyReused) {
		t.Fatalf("want ErrIdempotencyKeyReused, got %v", err)
	}
	if got := testutil.WalletBalance(t, e.Pool, w); got != 100_000 {
		t.Fatalf("balance %d", got)
	}
}

// TC-IDM-010: every mounted money-moving POST rejects a missing Idempotency-Key
// with 400 ERR_IDEMPOTENCY_KEY_REQUIRED, and the read/auth routes are untouched.
func TestIdempotency_MountedMoneyRoutesRequireKey(t *testing.T) {
	e := testkit.NewEnv(t, "")
	const secret = "integration-test-secret-0123456789"
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer rdb.Close()
	router := api.NewRouter(api.Deps{
		Cfg: config.Config{Env: "dev", Role: config.RoleAll, JWTSecret: secret,
			KYCStorageDir: t.TempDir(), StorageProvider: "local", MockBankSecret: "x"},
		Log: log, Pool: e.Pool, Redis: rdb,
	})
	uid, _, _ := e.UserWithMoney(t, 0)
	tok, err := auth.NewJWTIssuer(secret, "user").Issue(uid, "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path string, withKey bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		if withKey {
			req.Header.Set("Idempotency-Key", uuid.NewString())
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	for _, rt := range []struct{ method, path string }{
		{"POST", "/v1/transactions/request"},
		{"POST", "/v1/money-requests"},
		{"POST", "/v1/money-requests/split"},
		{"POST", "/v1/org/" + uuid.NewString() + "/transactions"},
	} {
		w := do(rt.method, rt.path, false)
		if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte("ERR_IDEMPOTENCY_KEY_REQUIRED")) {
			t.Errorf("%s %s without key: want 400 ERR_IDEMPOTENCY_KEY_REQUIRED, got %d %s", rt.method, rt.path, w.Code, w.Body)
		}
		if w := do(rt.method, rt.path, true); bytes.Contains(w.Body.Bytes(), []byte("ERR_IDEMPOTENCY_KEY_REQUIRED")) {
			t.Errorf("%s %s with key must pass the middleware, got %d %s", rt.method, rt.path, w.Code, w.Body)
		}
	}
	// reads must not demand a key
	if w := do("GET", "/v1/wallets", false); w.Code != http.StatusOK {
		t.Errorf("GET /v1/wallets: want 200, got %d", w.Code)
	}
	// non-money writes do not demand a key
	if w := do("POST", "/v1/money-requests/"+uuid.NewString()+"/decline", false); bytes.Contains(w.Body.Bytes(), []byte("ERR_IDEMPOTENCY_KEY_REQUIRED")) {
		t.Errorf("decline must not require a key: %d %s", w.Code, w.Body)
	}
}
