package org

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/listing"
	"github.com/hieumai1507/hpay-backend/internal/product"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

func TestOrgAdminOverview_ListsAllOrgsWithCounts(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	psvc := product.NewService(e.Pool)

	tag := uuid.NewString()[:8]
	f := testkit.NewOrgFixture(t, e, 0, []int{1, 2}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})
	orgName := "Org Admin Overview " + tag

	// newOrgFixture already named the org randomly; rename it to something we
	// can search for deterministically without colliding with other tests'
	// orgs in this shared dev database.
	if _, err := e.Pool.Exec(ctx, `UPDATE organizations SET name = $1 WHERE id = $2`, orgName, f.OrgID); err != nil {
		t.Fatalf("rename org: %v", err)
	}

	// A pending org disbursement (created by newOrgFixture's tiers via the
	// approval pause hook if funded) is not guaranteed here since fundAmount
	// is 0 — this test only asserts member/policy counts, which don't depend
	// on any pending transaction existing.

	rows, total, err := psvc.ListOrganizations(ctx, listing.Filter{Name: orgName})
	if err != nil {
		t.Fatalf("ListOrganizations: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("want exactly 1 match for %q, got total=%d rows=%d", orgName, total, len(rows))
	}
	row := rows[0]
	if row.OrgID != f.OrgID {
		t.Fatalf("org id: want %s, got %s", f.OrgID, row.OrgID)
	}
	// admin (creator) + maker + 2 checkers = 4 members.
	if row.MemberCount != 4 {
		t.Fatalf("member_count: want 4, got %d", row.MemberCount)
	}
	if row.PoliciesCount != 1 {
		t.Fatalf("policies_count: want 1, got %d", row.PoliciesCount)
	}
	if row.PendingApprovalsCount != 0 {
		t.Fatalf("pending_approvals_count: want 0, got %d", row.PendingApprovalsCount)
	}
}

func TestOrgAdminOverview_NotScopedToAnySingleCaller(t *testing.T) {
	// The whole point of this endpoint: unlike ListUserOrganizations(userID),
	// it takes no caller identity at all and still returns orgs nobody
	// querying it is a member of — proving it's a real admin-ops overview,
	// not a relabeled per-user query.
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	psvc := product.NewService(e.Pool)

	tag := uuid.NewString()[:8]
	f := testkit.NewOrgFixture(t, e, 0, nil, nil)
	orgName := "Org No Caller Scope " + tag
	if _, err := e.Pool.Exec(ctx, `UPDATE organizations SET name = $1 WHERE id = $2`, orgName, f.OrgID); err != nil {
		t.Fatalf("rename org: %v", err)
	}

	// No user_id / admin_id passed anywhere — service signature has none.
	rows, total, err := psvc.ListOrganizations(ctx, listing.Filter{Name: orgName})
	if err != nil {
		t.Fatalf("ListOrganizations: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].OrgID != f.OrgID {
		t.Fatalf("want the org to be visible with no caller-membership check, got total=%d rows=%d", total, len(rows))
	}
}
