package ledger

import (
	"testing"

	"github.com/google/uuid"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

func leg(w uuid.UUID, d store.EntryDirection, a int64) Leg {
	return Leg{WalletID: w, Direction: d, Amount: a}
}

func TestValidateLegs(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	cases := []struct {
		name    string
		legs    []Leg
		wantErr error
	}{
		{"balanced pair ok", []Leg{leg(a, store.EntryDirectionDebit, 100), leg(b, store.EntryDirectionCredit, 100)}, nil},
		{"three legs with fee ok", []Leg{leg(a, store.EntryDirectionDebit, 102), leg(b, store.EntryDirectionCredit, 100), leg(uuid.New(), store.EntryDirectionCredit, 2)}, nil},
		{"unbalanced", []Leg{leg(a, store.EntryDirectionDebit, 100), leg(b, store.EntryDirectionCredit, 99)}, ErrUnbalanced},
		{"single leg", []Leg{leg(a, store.EntryDirectionDebit, 100)}, ErrInvalidLeg},
		{"zero amount", []Leg{leg(a, store.EntryDirectionDebit, 0), leg(b, store.EntryDirectionCredit, 0)}, ErrInvalidLeg},
		{"negative amount", []Leg{leg(a, store.EntryDirectionDebit, -5), leg(b, store.EntryDirectionCredit, -5)}, ErrInvalidLeg},
	}
	for _, c := range cases {
		err := ValidateLegs(c.legs)
		if c.wantErr == nil && err != nil {
			t.Errorf("%s: want nil, got %v", c.name, err)
		}
		if c.wantErr != nil && (err == nil || !errorsIs(err, c.wantErr)) {
			t.Errorf("%s: want %v, got %v", c.name, c.wantErr, err)
		}
	}
}

// TestEdge_E14_SelfTransferRejected: money must move between two different wallets.
func TestEdge_E14_SelfTransferRejected(t *testing.T) {
	a := uuid.New()
	err := ValidateLegs([]Leg{leg(a, store.EntryDirectionDebit, 100), leg(a, store.EntryDirectionCredit, 100)})
	if err != ErrSelfTransfer {
		t.Fatalf("want ErrSelfTransfer, got %v", err)
	}
}

func errorsIs(err, target error) bool {
	for e := err; e != nil; {
		if e == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := e.(unwrapper)
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
