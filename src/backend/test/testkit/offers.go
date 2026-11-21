package testkit

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/offerguard"
)

func (e *Env) OfferCodeOf(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	var code string
	err := e.Pool.QueryRow(context.Background(),
		`SELECT o.code FROM user_offers uo JOIN offers o ON o.id = uo.offer_id WHERE uo.user_id = $1`, userID).Scan(&code)
	if err != nil {
		return ""
	}
	return code
}

func (e *Env) SetOffer(t *testing.T, userID uuid.UUID, code string) {
	t.Helper()
	if _, err := e.Pool.Exec(context.Background(), `
		INSERT INTO user_offers (user_id, offer_id, assigned_by)
		SELECT $1, id, 'test' FROM offers WHERE code = $2
		ON CONFLICT (user_id) DO UPDATE SET offer_id = EXCLUDED.offer_id, assigned_by = 'test'`, userID, code); err != nil {
		t.Fatal(err)
	}
}

func (e *Env) GuardCheck(userID uuid.UUID, amount int64) error {
	_, err := offerguard.New(e.Pool).Check(context.Background(), userID, "P2P_TRANSFER", "p2p", amount)
	return err
}
