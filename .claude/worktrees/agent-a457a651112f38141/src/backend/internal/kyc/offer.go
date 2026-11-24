package kyc

import (
	"context"

	"github.com/google/uuid"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// applyOfferForStatus moves the user's offer when the KYC status changes
// (docs/specs/w3-offer-limits, ADR-0007). It runs inside the same transaction as the
// status change. Verified or auto-approved: L0 becomes L1 (L1, L2 and merchant offers are
// kept). Rejected: any customer offer becomes L0. Pending and manual review change nothing.
func applyOfferForStatus(ctx context.Context, q *store.Queries, userID uuid.UUID, status store.KycStatus) error {
	switch status {
	case store.KycStatusVerified, store.KycStatusAutoApproved:
		return q.AssignKycApprovedOffer(ctx, userID)
	case store.KycStatusRejected:
		return q.AssignKycRejectedOffer(ctx, userID)
	}
	return nil
}
