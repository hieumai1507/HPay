package integration

// TC-IDM-011..013: migration 000015 on a database that already holds data.
// Runs against a scratch database so the shared test schema is never rolled back.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func migrationSQL(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMigration000015_UpDownBackfill(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	db := "mig15_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+db); err != nil {
		t.Skipf("cannot create scratch database: %v", err)
	}
	defer admin.Exec(ctx, "DROP DATABASE IF EXISTS "+db+" WITH (FORCE)")

	scfg := cfg.Copy()
	scfg.Database = db
	conn, err := pgx.ConnectConfig(ctx, scfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	ups, _ := filepath.Glob(filepath.Join("..", "..", "db", "migrations", "*.up.sql"))
	sort.Strings(ups)
	for _, f := range ups {
		base := filepath.Base(f)
		if strings.HasPrefix(base, "000015") {
			continue
		}
		if _, err := conn.Exec(ctx, migrationSQL(t, base)); err != nil {
			t.Fatalf("apply %s: %v", base, err)
		}
	}

	// legacy data: one user, two rows with different keys and one with NULL key
	var uid, uid2 uuid.UUID
	for i, p := range []*uuid.UUID{&uid, &uid2} {
		if err := conn.QueryRow(ctx, `INSERT INTO users (phone, password_hash, full_name) VALUES ($1,'x','Legacy') RETURNING id`,
			fmt.Sprintf("0911%07d", i+1)).Scan(p); err != nil {
			t.Fatal(err)
		}
	}
	ins := `INSERT INTO transactions (ref_code, idempotency_key, type, status, amount, initiated_by)
	        VALUES ($1, $2, 'p2p', 'completed', 1000, $3)`
	if _, err := conn.Exec(ctx, ins, "LEGACY-1", "legacy-key", uid); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, ins, "LEGACY-2", nil, uid); err != nil {
		t.Fatal(err)
	}
	// 000002 semantics before the migration: the same key from another user collides
	if _, err := conn.Exec(ctx, ins, "LEGACY-3", "legacy-key", uid2); err == nil {
		t.Fatal("precondition: global UNIQUE must reject a second user's identical key before 000015")
	}

	// ---- up on a populated database ----
	if _, err := conn.Exec(ctx, migrationSQL(t, "000015_idempotency_per_user.up.sql")); err != nil {
		t.Fatalf("000015 up on populated db: %v", err)
	}
	var hash *string
	if err := conn.QueryRow(ctx, `SELECT request_hash FROM transactions WHERE ref_code='LEGACY-1'`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash != nil {
		t.Fatalf("legacy rows must have NULL request_hash, got %q", *hash)
	}
	// per user: other user may now reuse the key; the same user may not
	if _, err := conn.Exec(ctx, ins, "NEW-1", "legacy-key", uid2); err != nil {
		t.Fatalf("other user must be able to use the same key after 000015: %v", err)
	}
	if _, err := conn.Exec(ctx, ins, "NEW-2", "legacy-key", uid); err == nil {
		t.Fatal("same user + same key must still be unique")
	}
	if _, err := conn.Exec(ctx, ins, "NEW-3", nil, uid); err != nil {
		t.Fatalf("NULL keys stay unconstrained: %v", err)
	}
	// request_hash column exists on money_requests too
	if _, err := conn.Exec(ctx, `SELECT idempotency_key, request_hash FROM money_requests LIMIT 1`); err != nil {
		t.Fatalf("money_requests columns: %v", err)
	}

	// ---- down: refuses to restore global uniqueness over colliding data, works once resolved ----
	if _, err := conn.Exec(ctx, migrationSQL(t, "000015_idempotency_per_user.down.sql")); err == nil {
		t.Fatal("down must fail loudly while two users share a key")
	}
	if _, err := conn.Exec(ctx, `DELETE FROM transactions WHERE ref_code='NEW-1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, migrationSQL(t, "000015_idempotency_per_user.down.sql")); err != nil {
		t.Fatalf("000015 down: %v", err)
	}
	if _, err := conn.Exec(ctx, `SELECT request_hash FROM transactions LIMIT 1`); err == nil {
		t.Fatal("down must drop transactions.request_hash")
	}
	// and up again (re-runnable)
	conn2, err := pgx.ConnectConfig(ctx, scfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close(ctx)
	if _, err := conn2.Exec(ctx, migrationSQL(t, "000015_idempotency_per_user.up.sql")); err != nil {
		t.Fatalf("000015 up after down: %v", err)
	}
}
