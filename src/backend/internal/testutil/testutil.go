// Package testutil provides shared helpers for integration tests.
// Tests using it MUST call PoolFromEnv first and skip when no DB is available.
package testutil

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

// RunSerialized is the TestMain body for packages that drive the outbox pump
// (internal/workflow, test/integration). Aborted past runs on the shared,
// persistent test database can leave unpublished outbox rows that clog the
// head of ClaimOutboxBatch's LIMIT window and starve drains — retire them
// once, up front, via a short-lived connection.
//
// Packages that pump the outbox must not run concurrently against the same
// database (two pumps can steal each other's events, especially with
// differently-wired engines — F6's pause hook). Run the suite with
// `go test -p 1 ./...` to serialize package test binaries; this function does
// not itself hold any lock for the run's duration.
func RunSerialized(m *testing.M) int {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		return m.Run() // tests will skip themselves via PoolFromEnv
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testutil: connect to purge stale outbox rows: %v\n", err)
		return 1
	}
	_, err = conn.Exec(ctx, "UPDATE outbox SET published_at = now() WHERE published_at IS NULL")
	conn.Close(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testutil: purge stale outbox rows: %v\n", err)
		return 1
	}
	return m.Run()
}

// PoolFromEnv connects to TEST_DATABASE_URL or skips the test.
// The database must already have migrations applied (CI does this; locally:
// docker compose up -d && make migrate-up, then set TEST_DATABASE_URL).
func PoolFromEnv(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse test db url: %v", err)
	}
	// TEST_DB_MAX_CONNS lets a developer reproduce a small CI runner (pgx defaults to
	// max(4, number of CPUs), so a 2-CPU runner gets 4 connections).
	if v := os.Getenv("TEST_DB_MAX_CONNS"); v != "" {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n < 1 {
			t.Fatalf("TEST_DB_MAX_CONNS must be a positive integer, got %q", v)
		}
		cfg.MaxConns = int32(n)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// NewUserWithWallet creates a user + main wallet and returns their ids.
func NewUserWithWallet(t *testing.T, pool *pgxpool.Pool) (userID, walletID uuid.UUID) {
	t.Helper()
	q := store.New(pool)
	phone := fmt.Sprintf("09%08d", rand.Intn(100_000_000))
	u, err := q.InsertUser(context.Background(), store.InsertUserParams{
		Phone: phone, PasswordHash: "x", FullName: "Test User",
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	w, err := q.CreateUserWallet(context.Background(), store.CreateUserWalletParams{
		OwnerID: u.ID, Currency: wallet.DefaultCurrency, Purpose: "main", Label: pgtype.Text{},
	})
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}
	return u.ID, w.ID
}

// SystemWallet returns the id of a seeded system wallet.
func SystemWallet(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	w, err := store.New(pool).GetSystemWallet(context.Background(), pgtype.Text{String: code, Valid: true})
	if err != nil {
		t.Fatalf("system wallet %s missing — run migrations: %v", code, err)
	}
	return w.ID
}

// WalletBalance reads the current balance.
func WalletBalance(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) int64 {
	t.Helper()
	w, err := store.New(pool).GetWalletByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get wallet: %v", err)
	}
	return w.Balance
}

// LedgerSum returns the ledger-derived balance (recon invariant).
func LedgerSum(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) int64 {
	t.Helper()
	sum, err := store.New(pool).SumLedgerByWallet(context.Background(), id)
	if err != nil {
		t.Fatalf("sum ledger: %v", err)
	}
	return sum
}
