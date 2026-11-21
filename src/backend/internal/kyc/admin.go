package kyc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/listing"
	"github.com/hieumai1507/hpay-backend/internal/storage"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

const signedURLTTL = 5 * time.Minute // docs 01 §3.4

var (
	ErrKYCNotFound     = errors.New("kyc request not found")
	ErrAlreadyReviewed = errors.New("kyc request already reviewed")
)

// AdminService is the review side: queue + decision (docs 00 §8.2 manual band).
type AdminService struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	signer *storage.Signer
}

func NewAdminService(pool *pgxpool.Pool, signer *storage.Signer) *AdminService {
	return &AdminService{pool: pool, q: store.New(pool), signer: signer}
}

type QueueItem struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	FullName  string    `json:"full_name"`
	Phone     string    `json:"phone"`
	IDCardUrl string    `json:"id_card_url"` // signed, 5-minute TTL
	SelfieUrl string    `json:"selfie_url"`  // signed, 5-minute TTL
	AiScore   int       `json:"ai_score"`
	AiVerdict string    `json:"ai_verdict"`
	AiReasons any       `json:"ai_reasons,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Queue lists requests by tab; image links come back HMAC-signed so the SPA
// can render <img src> directly.
func (s *AdminService) Queue(ctx context.Context, tab string, f listing.Filter) ([]QueueItem, int64, error) {
	var status store.KycStatus
	switch tab {
	case "", "manual":
		status = store.KycStatusManualReview
	case "auto":
		status = store.KycStatusAutoApproved
	case "rejected":
		status = store.KycStatusRejected
	default:
		return nil, 0, errors.New("tab must be manual|auto|rejected")
	}
	f = f.Normalize()
	rows, err := s.q.ListKYCQueue(ctx, store.ListKYCQueueParams{
		Status:    status,
		Name:      f.Text(),
		FromAt:    f.FromTS(),
		ToAt:      f.ToTS(),
		SortOrder: f.SortOrder(),
		Lim:       f.Limit,
		Off:       f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountKYCQueue(ctx, store.CountKYCQueueParams{
		Status: status, Name: f.Text(), FromAt: f.FromTS(), ToAt: f.ToTS(),
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]QueueItem, 0, len(rows))
	for _, r := range rows {
		it := QueueItem{
			ID: r.ID, UserID: r.UserID, FullName: r.FullName, Phone: r.Phone,
			IDCardUrl: s.signer.SignURL(r.IDCardUrl, signedURLTTL),
			SelfieUrl: s.signer.SignURL(r.SelfieUrl, signedURLTTL),
			AiScore:   int(r.AiScore), AiVerdict: r.AiVerdict,
			Status: string(r.Status), CreatedAt: r.CreatedAt.Time,
		}
		if len(r.AiReasons) > 0 {
			it.AiReasons = jsonRaw(r.AiReasons)
		}
		items = append(items, it)
	}
	return items, total, nil
}

type ReviewResult struct {
	Status string `json:"status"`
}

// Review finalizes a manual_review request. The guarded UPDATE is the race
// arbiter: two admins deciding simultaneously — the second matches 0 rows and
// gets ErrAlreadyReviewed (409, groundwork for admin edge A01).
func (s *AdminService) Review(ctx context.Context, adminID, requestID uuid.UUID, approve bool, reason string) (ReviewResult, error) {
	newStatus := store.KycStatusRejected
	action := "kyc.reject"
	if approve {
		newStatus = store.KycStatusVerified
		action = "kyc.approve"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReviewResult{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	n, err := qtx.ReviewKYCGuarded(ctx, store.ReviewKYCGuardedParams{
		ID: requestID, Status: newStatus, ReviewerID: adminID,
	})
	if err != nil {
		return ReviewResult{}, err
	}
	if n == 0 {
		if _, gerr := s.q.GetKYCRequest(ctx, requestID); gerr != nil {
			if errors.Is(gerr, pgx.ErrNoRows) {
				return ReviewResult{}, ErrKYCNotFound
			}
			return ReviewResult{}, gerr
		}
		return ReviewResult{}, ErrAlreadyReviewed
	}
	req, err := qtx.GetKYCRequest(ctx, requestID)
	if err != nil {
		return ReviewResult{}, err
	}
	if _, err := qtx.SetUserKycFromManualReview(ctx, store.SetUserKycFromManualReviewParams{
		ID: req.UserID, KycStatus: newStatus,
	}); err != nil {
		return ReviewResult{}, err
	}
	if err := qtx.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: adminID, ActorType: "admin", Action: action,
		Resource: "kyc_request " + requestID.String(),
		Reason:   pgtype.Text{String: reason, Valid: reason != ""},
	}); err != nil {
		return ReviewResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReviewResult{}, err
	}
	return ReviewResult{Status: string(newStatus)}, nil
}
