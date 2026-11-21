package wallet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// Idempotency (constitution C-03, ADR-0011): a key is scoped to the user who
// sent it and bound to the request that first used it.
var (
	// ErrIdempotencyKeyRequired: a money-moving write reached the service without a key.
	ErrIdempotencyKeyRequired = errors.New("idempotency key required")
	// ErrIdempotencyKeyReused: the key was already used by this user for a different request.
	ErrIdempotencyKeyReused = errors.New("idempotency key already used with a different request")
)

// RequestHash is the SHA-256 (hex) of a canonical, length-prefixed encoding of
// the fields that define an operation. Length prefixes make the encoding
// unambiguous ("ab","c" never collides with "a","bc").
func RequestHash(op string, fields ...string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d:%s;", len(op), op)
	for _, f := range fields {
		fmt.Fprintf(h, "%d:%s;", len(f), f)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// HashText wraps a hash for the nullable request_hash column.
func HashText(hash string) pgtype.Text { return pgtype.Text{String: hash, Valid: hash != ""} }

// KeyText wraps an idempotency key for the nullable idempotency_key column.
func KeyText(key string) pgtype.Text { return pgtype.Text{String: key, Valid: key != ""} }

// HashMatches treats a NULL stored hash (rows from before migration 000015) as
// "unknown, accept replay".
func HashMatches(stored pgtype.Text, hash string) bool {
	return !stored.Valid || stored.String == hash
}

// FindReplay looks up an earlier transaction of THIS user with the same key.
// (nil, nil) means the key is unused. A different request hash returns
// ErrIdempotencyKeyReused. Another user's transaction is never visible here.
func FindReplay(ctx context.Context, q *store.Queries, userID uuid.UUID, key, hash string) (*store.GetUserTransactionByIdempotencyKeyRow, error) {
	if key == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	old, err := q.GetUserTransactionByIdempotencyKey(ctx, store.GetUserTransactionByIdempotencyKeyParams{
		InitiatedBy: userID, IdempotencyKey: KeyText(key),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !HashMatches(old.RequestHash, hash) {
		return nil, ErrIdempotencyKeyReused
	}
	return &old, nil
}

// IsIdempotencyConflict reports whether err is the per-user unique violation
// raised by a concurrent insert with the same key.
func IsIdempotencyConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "idempotency")
}
