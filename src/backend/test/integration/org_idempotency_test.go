package integration

// ADR-0011 for the org package: a key is scoped to the acting user and bound to the request.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/org"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

func orgIdemFixture(t *testing.T, e *env) *orgFixture {
	return newOrgFixture(t, e, 20_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})
}

func (e *env) countOrgTxnsByKey(t *testing.T, key string) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM transactions WHERE idempotency_key=$1`, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestOrgDisbursement_SameKeySameRequestReplays(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := orgIdemFixture(t, e)
	key := uuid.NewString()

	a, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "pay", key)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "pay", key)
	if err != nil {
		t.Fatalf("a replay must succeed: %v", err)
	}
	if !b.Duplicate || b.RefCode != a.RefCode {
		t.Fatalf("replay must return the first transaction: %+v vs %+v", b, a)
	}
	if n := e.countOrgTxnsByKey(t, key); n != 1 {
		t.Fatalf("want 1 transaction for the key, got %d", n)
	}
}

func TestOrgDisbursement_SameKeyDifferentRequestRefused(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := orgIdemFixture(t, e)
	key := uuid.NewString()
	if _, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "pay", key); err != nil {
		t.Fatal(err)
	}
	if _, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 2_000_000, "pay", key); !errors.Is(err, wallet.ErrIdempotencyKeyReused) {
		t.Fatalf("a different amount under the same key must be refused, got %v", err)
	}
	if n := e.countOrgTxnsByKey(t, key); n != 1 {
		t.Fatalf("want 1 transaction for the key, got %d", n)
	}
}

// Another maker using the same key string must neither see nor be blocked by the first one's transaction.
func TestOrgDisbursement_KeyIsPerUser(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := orgIdemFixture(t, e)
	maker2, _, phone2 := e.userWithMoney(t, 0)
	if err := e.osvc.AddMember(ctx, f.adminID, f.orgID, phone2, "maker", 0); err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	a, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "pay", key)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.osvc.CreateDisbursement(ctx, maker2, f.orgID, f.receiverPhone, 3_000_000, "other", key)
	if err != nil {
		t.Fatalf("another user's key must be independent: %v", err)
	}
	if b.Duplicate || b.RefCode == a.RefCode || b.Amount != 3_000_000 {
		t.Fatalf("second maker must get its own transaction, got %+v", b)
	}
}

func TestOrgDisbursement_MissingKeyRefused(t *testing.T) {
	e := newEnv(t, "")
	f := orgIdemFixture(t, e)
	if _, err := e.osvc.CreateDisbursement(context.Background(), f.makerID, f.orgID, f.receiverPhone, 1_000_000, "", ""); !errors.Is(err, wallet.ErrIdempotencyKeyRequired) {
		t.Fatalf("want ErrIdempotencyKeyRequired, got %v", err)
	}
}

func TestOrgDisbursement_ConcurrentSameKeyCreatesOne(t *testing.T) {
	e := newEnv(t, "")
	f := orgIdemFixture(t, e)
	key := uuid.NewString()
	var wg sync.WaitGroup
	errs := make([]error, 6)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = e.osvc.CreateDisbursement(context.Background(), f.makerID, f.orgID, f.receiverPhone, 1_000_000, "pay", key)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("call %d: %v", i, err)
		}
	}
	if n := e.countOrgTxnsByKey(t, key); n != 1 {
		t.Fatalf("want exactly 1 transaction, got %d", n)
	}
}

func TestOrgTopup_ReplayAndReuse(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := orgIdemFixture(t, e)
	key := uuid.NewString()
	a, err := e.osvc.Topup(ctx, f.adminID, f.orgID, 500_000, key)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.osvc.Topup(ctx, f.adminID, f.orgID, 500_000, key)
	if err != nil || !b.Duplicate || b.RefCode != a.RefCode {
		t.Fatalf("replay must return the first topup: %+v %v", b, err)
	}
	if _, err := e.osvc.Topup(ctx, f.adminID, f.orgID, 900_000, key); !errors.Is(err, wallet.ErrIdempotencyKeyReused) {
		t.Fatalf("a different amount under the same key must be refused, got %v", err)
	}
	if _, err := e.osvc.Topup(ctx, f.adminID, f.orgID, 500_000, ""); !errors.Is(err, wallet.ErrIdempotencyKeyRequired) {
		t.Fatalf("a missing key must be refused, got %v", err)
	}
	var credited int64
	if err := e.pool.QueryRow(ctx, `SELECT coalesce(sum(amount),0) FROM transactions WHERE idempotency_key=$1 AND status='completed'`, key).Scan(&credited); err != nil {
		t.Fatal(err)
	}
	if credited != 500_000 {
		t.Fatalf("the org must be credited once, got %d", credited)
	}
}

var _ = org.ErrNotMember
