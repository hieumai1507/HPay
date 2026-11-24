package org

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/org"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

func TestOrgAdminPolicy_CreatePolicyAsAdmin_NoOrgMembershipRequired(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 0, nil, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	adminID := uuid.New() // deliberately NOT an org_members row for this org
	// newOrgFixture already installed a v1 "default" policy — reuse that
	// name so this call exercises the real "bump an existing policy"
	// versioning path, not just "create a brand-new name at v1".
	res, err := e.Osvc.CreatePolicyAsAdmin(ctx, adminID, f.OrgID, "default", 10_000_000, nil,
		[]approval.Tier{{Level: 1, RequiredApprovals: 1}, {Level: 2, RequiredApprovals: 2}})
	if err != nil {
		t.Fatalf("CreatePolicyAsAdmin: %v", err)
	}
	if res.Version != 2 {
		t.Fatalf("want version 2 (fixture already created v1), got %d", res.Version)
	}
}

// The service always computes "next version" fresh (SELECT MAX+1), so a
// plain `go func` race against the real service — even with a start-channel
// barrier — reliably fails to collide: each call is several sequential
// round-trips (BEGIN/SELECT/UPDATE/INSERT/INSERT audit/COMMIT), plenty of
// time for the local Postgres connection to let one finish before the other
// begins. This test forces the exact interleaving the constraint has to
// survive instead: two manual, uncommitted transactions both insert
// version=2 for the same (org, name); Postgres blocks the second INSERT
// until the first's fate (commit/rollback) is known, so the sequencing
// below is deterministic, not timing-luck.
func TestOrgAdminPolicy_ConcurrentInsertsRaceToOneWinner(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 0, nil, []approval.Tier{{Level: 1, RequiredApprovals: 1}}) // installs v1 "default"

	insertV2 := func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO approval_policies (org_id, name, min_amount, tiers, version, created_by)
			VALUES ($1, 'default', 0, '[]'::jsonb, 2, $2)`, f.OrgID, uuid.New())
		return err
	}

	tx1, err := e.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	if err := insertV2(tx1); err != nil {
		t.Fatalf("tx1 insert v2: %v", err)
	}

	// tx2's insert of the same (org, name, 2) blocks on tx1's uncommitted
	// row until tx1 resolves — run it in a goroutine and confirm it's still
	// pending before committing tx1.
	tx2, err := e.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx2.Rollback(ctx)
	tx2Done := make(chan error, 1)
	go func() { tx2Done <- insertV2(tx2) }()

	select {
	case err := <-tx2Done:
		t.Fatalf("tx2's insert should have blocked on tx1's uncommitted row, but it returned immediately: %v", err)
	case <-time.After(300 * time.Millisecond):
		// still blocked, as expected
	}

	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("tx1 commit: %v", err)
	}

	select {
	case err := <-tx2Done:
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			t.Fatalf("want a 23505 unique_violation on approval_policies_org_name_version_key once tx1 commits, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tx2 never resolved after tx1 committed")
	}
}

// Same deterministic blocking-transaction technique, but this time the
// second side goes through the real service (not manual SQL), proving
// createPolicy's unique-violation → ErrPolicyVersionConflict mapping fires
// under a genuine collision — not just that the raw constraint exists.
func TestOrgAdminPolicy_VersionConflictMapsToSentinelError(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 0, nil, []approval.Tier{{Level: 1, RequiredApprovals: 1}}) // installs v1 "default"

	// Uncommitted tx holding version=2 — invisible to other transactions
	// under READ COMMITTED, so the service below still computes next=2 too.
	tx1, err := e.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	if _, err := tx1.Exec(ctx, `
		INSERT INTO approval_policies (org_id, name, min_amount, tiers, version, created_by)
		VALUES ($1, 'default', 0, '[]'::jsonb, 2, $2)`, f.OrgID, uuid.New()); err != nil {
		t.Fatalf("tx1 insert v2: %v", err)
	}

	svcDone := make(chan error, 1)
	go func() {
		_, err := e.Osvc.CreatePolicyAsAdmin(ctx, uuid.New(), f.OrgID, "default", 10_000_000, nil,
			[]approval.Tier{{Level: 1, RequiredApprovals: 1}})
		svcDone <- err
	}()

	select {
	case err := <-svcDone:
		t.Fatalf("CreatePolicyAsAdmin should have blocked on tx1's uncommitted row, but returned immediately: %v", err)
	case <-time.After(300 * time.Millisecond):
		// still blocked, as expected
	}

	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("tx1 commit: %v", err)
	}

	select {
	case err := <-svcDone:
		if !testkit.ErrIs(err, org.ErrPolicyVersionConflict) {
			t.Fatalf("want ErrPolicyVersionConflict once tx1 commits out from under it, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CreatePolicyAsAdmin never resolved after tx1 committed")
	}
}
