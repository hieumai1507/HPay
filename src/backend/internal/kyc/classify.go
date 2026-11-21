package kyc

import store "github.com/hieumai1507/hpay-backend/internal/store/gen"

// Thresholds from docs 00 §8.2: >=85 auto-approve, 50-84 human review, <50 reject.
const (
	autoApproveAt  = 85
	manualReviewAt = 50
)

// Verdict is the AI recommendation persisted to kyc_requests.ai_verdict.
type Verdict string

const (
	VerdictAutoApprove  Verdict = "auto_approve"
	VerdictManualReview Verdict = "manual_review"
	VerdictAutoReject   Verdict = "auto_reject"
)

// Classify maps a score to the verdict and the kyc status both tables move to.
// Pure and deterministic — the LLM only produced the score, this decides.
func Classify(score int) (Verdict, store.KycStatus) {
	switch {
	case score >= autoApproveAt:
		return VerdictAutoApprove, store.KycStatusAutoApproved
	case score >= manualReviewAt:
		return VerdictManualReview, store.KycStatusManualReview
	default:
		return VerdictAutoReject, store.KycStatusRejected
	}
}
