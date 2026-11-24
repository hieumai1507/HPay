package admin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/listing"
	"github.com/hieumai1507/hpay-backend/internal/product"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// Audit log server-side search (docs plan audit-log-search): ListAuditLogs
// now honors listing.Filter the same way fraud flags and workflow instances
// do — free-text search across action/resource/reason plus a created_at
// range — instead of only slicing by limit/offset.
//
// audit_logs is a real, never-truncated Postgres table shared across every
// test run in this session, so every search needle below embeds a fresh
// uuid — a literal string like "POLICY_UPDATE" would accumulate matches from
// earlier runs and make exact-count assertions flaky.

func TestAuditLogSearch_MatchesAction(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	svc := product.NewService(e.Pool)
	actor := uuid.New()
	tag := uuid.NewString()[:8]
	action := "POLICY_UPDATE_" + tag

	if err := svc.RecordAudit(ctx, actor, "admin", action, "OrgPolicy#1", "raised daily limit"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordAudit(ctx, actor, "admin", "USER_LOCK_"+tag, "User#2", "suspicious activity"); err != nil {
		t.Fatal(err)
	}

	rows, total, err := svc.ListAuditLogs(ctx, listing.Filter{Name: action})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("want exactly 1 match, got total=%d rows=%d", total, len(rows))
	}
	if rows[0].Action != action {
		t.Fatalf("want %s, got %s", action, rows[0].Action)
	}
}

func TestAuditLogSearch_MatchesResourceAndReason(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	svc := product.NewService(e.Pool)
	actor := uuid.New()
	tag := uuid.NewString()[:8]
	resource := "KYCReview#" + tag
	reason := "score above threshold " + tag

	if err := svc.RecordAudit(ctx, actor, "admin", "KYC_APPROVE_"+tag, resource, reason); err != nil {
		t.Fatal(err)
	}

	// Matches via resource, not action.
	_, total, err := svc.ListAuditLogs(ctx, listing.Filter{Name: resource})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("resource match: want 1, got %d", total)
	}

	// Matches via reason, not action or resource.
	_, total, err = svc.ListAuditLogs(ctx, listing.Filter{Name: "threshold " + tag})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("reason match: want 1, got %d", total)
	}
}

func TestAuditLogSearch_NoMatchReturnsEmpty(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	svc := product.NewService(e.Pool)
	tag := uuid.NewString()[:8]

	if err := svc.RecordAudit(ctx, uuid.New(), "admin", "POLICY_UPDATE_"+tag, "OrgPolicy#1", "raised daily limit"); err != nil {
		t.Fatal(err)
	}

	rows, total, err := svc.ListAuditLogs(ctx, listing.Filter{Name: "no-such-needle-" + tag})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(rows) != 0 {
		t.Fatalf("want no matches, got total=%d rows=%d", total, len(rows))
	}
}

func TestAuditLogSearch_DateRangeFilters(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	svc := product.NewService(e.Pool)
	actor := uuid.New()
	marker := "SEED_MARKER_" + uuid.NewString()[:8]

	if err := svc.RecordAudit(ctx, actor, "admin", marker, "Marker#1", ""); err != nil {
		t.Fatal(err)
	}

	future := time.Now().Add(24 * time.Hour)
	_, total, err := svc.ListAuditLogs(ctx, listing.Filter{Name: marker, From: &future})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("from in the future must exclude the row just inserted, got total=%d", total)
	}

	past := time.Now().Add(-24 * time.Hour)
	_, total, err = svc.ListAuditLogs(ctx, listing.Filter{Name: marker, From: &past})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("from in the past must include the row, got total=%d", total)
	}
}
