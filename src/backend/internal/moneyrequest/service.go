package moneyrequest

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/fee"
	"github.com/hieumai1507/hpay-backend/internal/ledger"
	"github.com/hieumai1507/hpay-backend/internal/offerguard"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

const requestTTL = 72 * time.Hour

var (
	ErrSelfRequest     = errors.New("cannot request money from yourself")
	ErrPayerNotFound   = errors.New("payer not found")
	ErrRequestNotFound = errors.New("money request not found")
	ErrNotPayable      = errors.New("money request is not payable")
	ErrExpired         = errors.New("money request expired")
	ErrForbidden       = errors.New("not allowed to act on this money request")
	ErrRemindTooSoon   = errors.New("money request reminded too soon")
)

type Service struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	engine *workflow.Engine
	fees   *fee.Service
	guard  *offerguard.Guard
}

func NewService(pool *pgxpool.Pool, engine *workflow.Engine, fees *fee.Service) *Service {
	return &Service{pool: pool, q: store.New(pool), engine: engine, fees: fees, guard: offerguard.New(pool)}
}

type Result struct {
	ID        uuid.UUID `json:"id"`
	Amount    int64     `json:"amount"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Create makes a single request (requester asks payer for amount).
func (s *Service) Create(ctx context.Context, requesterID uuid.UUID, payerPhone string, amount int64, note, idemKey string) (Result, error) {
	if idemKey == "" {
		return Result{}, wallet.ErrIdempotencyKeyRequired
	}
	if amount <= 0 {
		return Result{}, wallet.ErrAmountInvalid
	}
	payer, err := s.q.GetUserByPhone(ctx, payerPhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrPayerNotFound
		}
		return Result{}, err
	}
	if payer.ID == requesterID {
		return Result{}, ErrSelfRequest
	}
	hash := wallet.RequestHash("money_request_create", payer.ID.String(), strconv.FormatInt(amount, 10), note)
	if prev, err := s.replay(ctx, s.q, requesterID, idemKey, hash); err != nil || prev != nil {
		if err != nil {
			return Result{}, err
		}
		return prev.Requests[0], nil
	}
	row, err := s.q.InsertMoneyRequest(ctx, store.InsertMoneyRequestParams{
		RequesterID: requesterID, PayerID: payer.ID, Amount: amount,
		Note: noteText(note), SplitGroupID: uuid.Nil,
		ExpiresAt:      ts(time.Now().Add(requestTTL)),
		IdempotencyKey: wallet.KeyText(idemKey), RequestHash: wallet.HashText(hash),
	})
	if err != nil {
		if wallet.IsIdempotencyConflict(err) { // concurrent request with the same key won the race
			if prev, rerr := s.replay(ctx, s.q, requesterID, idemKey, hash); rerr != nil || prev != nil {
				if rerr != nil {
					return Result{}, rerr
				}
				return prev.Requests[0], nil
			}
		}
		return Result{}, err
	}
	return Result{ID: row.ID, Amount: row.Amount, Status: string(row.Status), ExpiresAt: row.ExpiresAt.Time}, nil
}

// replay returns the money request(s) this requester already created with the
// key (nil when the key is unused) or ErrIdempotencyKeyReused when the stored
// hash differs. A NULL stored hash is accepted (pre-000015 rows).
func (s *Service) replay(ctx context.Context, q *store.Queries, requesterID uuid.UUID, idemKey, hash string) (*SplitResult, error) {
	rows, err := q.ListMoneyRequestsByIdempotencyKey(ctx, store.ListMoneyRequestsByIdempotencyKeyParams{
		RequesterID: requesterID, IdempotencyKey: wallet.KeyText(idemKey),
	})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	if !wallet.HashMatches(rows[0].RequestHash, hash) {
		return nil, wallet.ErrIdempotencyKeyReused
	}
	out := &SplitResult{SplitGroupID: rows[0].SplitGroupID}
	for _, r := range rows {
		out.Requests = append(out.Requests, Result{ID: r.ID, Amount: r.Amount, Status: string(r.Status), ExpiresAt: r.ExpiresAt.Time})
	}
	return out, nil
}

type SplitItem struct {
	PayerID uuid.UUID
	Amount  int64
}

type SplitResult struct {
	SplitGroupID uuid.UUID `json:"split_group_id"`
	Requests     []Result  `json:"requests"`
}

// CreateSplit fans out N requests under one split_group_id. amounts, when
// given, must already sum to totalAmount (validated by the caller/handler via
// ValidateCustomSplit) — when nil, shares are computed with SplitEven.
func (s *Service) CreateSplit(ctx context.Context, requesterID uuid.UUID, payerPhones []string, totalAmount int64, note string, amounts []int64, idemKey string) (SplitResult, error) {
	if idemKey == "" {
		return SplitResult{}, wallet.ErrIdempotencyKeyRequired
	}
	if totalAmount <= 0 {
		return SplitResult{}, wallet.ErrAmountInvalid
	}
	if len(payerPhones) == 0 {
		return SplitResult{}, ErrBadSplit
	}
	shares := amounts
	if shares == nil {
		var err error
		shares, err = SplitEven(totalAmount, len(payerPhones))
		if err != nil {
			return SplitResult{}, err
		}
	} else {
		if len(shares) != len(payerPhones) {
			return SplitResult{}, errors.New("amounts length must match payer_phones length")
		}
		if err := ValidateCustomSplit(shares, totalAmount); err != nil {
			return SplitResult{}, err
		}
	}

	payerIDs := make([]uuid.UUID, len(payerPhones))
	for i, phone := range payerPhones {
		payer, err := s.q.GetUserByPhone(ctx, phone)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return SplitResult{}, ErrPayerNotFound
			}
			return SplitResult{}, err
		}
		if payer.ID == requesterID {
			return SplitResult{}, ErrSelfRequest
		}
		payerIDs[i] = payer.ID
	}

	hashParts := []string{strconv.FormatInt(totalAmount, 10), note}
	for i, id := range payerIDs {
		hashParts = append(hashParts, id.String(), strconv.FormatInt(shares[i], 10))
	}
	hash := wallet.RequestHash("money_request_split", hashParts...)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SplitResult{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	// A split writes several rows under one key, so the unique index cannot
	// serialize concurrent retries; a per-(requester,key) advisory lock does.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, requesterID.String()+":"+idemKey); err != nil {
		return SplitResult{}, err
	}
	prev, err := s.replay(ctx, qtx, requesterID, idemKey, hash)
	if err != nil {
		return SplitResult{}, err
	}
	if prev != nil {
		return *prev, nil
	}

	groupID := uuid.New()
	expiresAt := ts(time.Now().Add(requestTTL))
	results := make([]Result, len(payerIDs))
	for i, payerID := range payerIDs {
		row, err := qtx.InsertMoneyRequest(ctx, store.InsertMoneyRequestParams{
			RequesterID: requesterID, PayerID: payerID, Amount: shares[i],
			Note: noteText(note), SplitGroupID: groupID,
			ExpiresAt:      expiresAt,
			IdempotencyKey: wallet.KeyText(idemKey), RequestHash: wallet.HashText(hash),
		})
		if err != nil {
			return SplitResult{}, err
		}
		results[i] = Result{ID: row.ID, Amount: row.Amount, Status: string(row.Status), ExpiresAt: row.ExpiresAt.Time}
	}
	if err := tx.Commit(ctx); err != nil {
		return SplitResult{}, err
	}
	return SplitResult{SplitGroupID: groupID, Requests: results}, nil
}

type PayResult struct {
	RefCode   string    `json:"ref_code"`
	Status    string    `json:"status"`
	Amount    int64     `json:"amount"`
	Fee       int64     `json:"fee"`
	Total     int64     `json:"total"`
	CreatedAt time.Time `json:"created_at"`
	Duplicate bool      `json:"duplicate,omitempty"`
}

// Pay is the payer's action: lock the request row, verify it is still
// payable RIGHT NOW (E19 — the expires_at check closes the race, the row
// lock makes it atomic against a concurrent sweeper), create the underlying
// P2P transaction and start the engine, then finalize — all in one DB tx so
// a request is never left "paid" without a transaction or vice versa.
func (s *Service) Pay(ctx context.Context, payerID, requestID uuid.UUID, idemKey string) (PayResult, error) {
	if idemKey == "" {
		return PayResult{}, wallet.ErrIdempotencyKeyRequired
	}
	hash := wallet.RequestHash("money_request_pay", requestID.String())
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayResult{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	req, err := qtx.GetMoneyRequestForUpdate(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PayResult{}, ErrRequestNotFound
		}
		return PayResult{}, err
	}
	if req.PayerID != payerID {
		return PayResult{}, ErrForbidden
	}
	// Replay check runs under the request row lock: a concurrent retry with the
	// same key waits here, then sees the transaction the first call committed.
	old, err := wallet.FindReplay(ctx, qtx, payerID, idemKey, hash)
	if err != nil {
		return PayResult{}, err
	}
	if old != nil {
		return PayResult{RefCode: old.RefCode, Status: string(old.Status), Amount: old.Amount,
			Fee: old.Fee, Total: old.Amount + old.Fee, CreatedAt: old.CreatedAt.Time, Duplicate: true}, nil
	}
	if req.Status != store.MoneyRequestStatusPending {
		return PayResult{}, ErrNotPayable
	}
	if !req.ExpiresAt.Time.After(time.Now()) {
		return PayResult{}, ErrExpired // E19
	}

	payerWallet, err := qtx.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: payerID, Currency: wallet.DefaultCurrency})
	if err != nil {
		return PayResult{}, wallet.ErrWalletNotFound
	}
	requesterWallet, err := qtx.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: req.RequesterID, Currency: wallet.DefaultCurrency})
	if err != nil {
		return PayResult{}, wallet.ErrWalletNotFound
	}
	payer, err := qtx.GetUserStatusByID(ctx, payerID)
	if err != nil {
		return PayResult{}, err
	}
	// Read the offer limits and the fee on THIS transaction's connection: asking the pool for
	// a second connection while this one is held can deadlock a small pool under load.
	feeAmt, err := fee.NewServiceWithQueries(qtx).For(ctx, store.TxnTypeP2p, payer.KycStatus, req.Amount)
	if err != nil {
		return PayResult{}, err
	}
	// BUG-0021: same rule as transfer.ExecuteEngineConfirm (BUG-0017). Serialize this payer's
	// starts with a transaction lock (taken after the request row lock, released at commit, after
	// the insert that makes this amount count), then check the per-transaction cap, the daily and
	// monthly volume and the interval throttle. Everything runs on this transaction's connection.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('limit-window:' || $1::text, 0))`, payerID); err != nil {
		return PayResult{}, err
	}
	if err := offerguard.NewWithQueries(qtx).CheckWindows(ctx, payerID, "MONEY_REQUEST", store.TxnTypeP2p, req.Amount, feeAmt); err != nil {
		return PayResult{}, err
	}

	payerBalance, err := qtx.GetWalletByID(ctx, payerWallet.ID)
	if err != nil {
		return PayResult{}, err
	}
	if payerBalance.Balance < req.Amount+feeAmt {
		return PayResult{}, ledger.ErrInsufficientBalance // soft; hard check in MOVE_FUNDS
	}

	txn, err := qtx.InsertTransaction(ctx, store.InsertTransactionParams{
		RefCode: wallet.NewRefCode(), IdempotencyKey: wallet.KeyText(idemKey),
		Type:           store.TxnTypeP2p,
		SenderWalletID: payerWallet.ID, ReceiverWalletID: requesterWallet.ID,
		Amount: req.Amount, Fee: feeAmt,
		Note: noteText(req.Note.String), InitiatedBy: payerID,
		RequestHash: wallet.HashText(hash),
	})
	if err != nil {
		// The key was used by this user between the check above and the insert,
		// for a different money request (the request row lock rules out the
		// same one): reject, the transaction is aborted so no lookup is possible.
		if wallet.IsIdempotencyConflict(err) {
			return PayResult{}, wallet.ErrIdempotencyKeyReused
		}
		return PayResult{}, err
	}
	if _, err := s.engine.Start(ctx, tx, "p2p_transfer", txn.ID, map[string]any{"ref_code": txn.RefCode}); err != nil {
		return PayResult{}, err
	}
	n, err := qtx.FinalizeMoneyRequestPaid(ctx, store.FinalizeMoneyRequestPaidParams{ID: requestID, PaidTxnID: txn.ID})
	if err != nil {
		return PayResult{}, err
	}
	if n == 0 {
		// Lost the race to the expiry sweeper between our initial check and
		// here — impossible under the row lock we hold, but guard anyway
		// rather than silently commit an inconsistent state.
		return PayResult{}, ErrExpired
	}
	if err := tx.Commit(ctx); err != nil {
		return PayResult{}, err
	}
	return PayResult{RefCode: txn.RefCode, Status: string(txn.Status), Amount: req.Amount,
		Fee: feeAmt, Total: req.Amount + feeAmt, CreatedAt: txn.CreatedAt.Time}, nil
}

// Decline is the payer's rejection.
func (s *Service) Decline(ctx context.Context, payerID, requestID uuid.UUID) error {
	n, err := s.q.DeclineMoneyRequest(ctx, store.DeclineMoneyRequestParams{ID: requestID, PayerID: payerID})
	if err != nil {
		return err
	}
	if n == 0 {
		return s.declineOrCancelError(ctx, requestID, payerID, true)
	}
	return nil
}

// Cancel is the requester's withdrawal.
func (s *Service) Cancel(ctx context.Context, requesterID, requestID uuid.UUID) error {
	n, err := s.q.CancelMoneyRequest(ctx, store.CancelMoneyRequestParams{ID: requestID, RequesterID: requesterID})
	if err != nil {
		return err
	}
	if n == 0 {
		return s.declineOrCancelError(ctx, requestID, requesterID, false)
	}
	return nil
}

// declineOrCancelError distinguishes "not found", "wrong actor" and "wrong
// state" for a clean error contract instead of a single generic failure.
func (s *Service) declineOrCancelError(ctx context.Context, requestID, actorID uuid.UUID, actorIsPayer bool) error {
	req, err := s.q.GetMoneyRequest(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRequestNotFound
		}
		return err
	}
	owner := req.RequesterID
	if actorIsPayer {
		owner = req.PayerID
	}
	if owner != actorID {
		return ErrForbidden
	}
	return ErrNotPayable
}

type InboxItem = store.ListInboxPendingRow

func (s *Service) Inbox(ctx context.Context, payerID uuid.UUID) ([]InboxItem, error) {
	return s.q.ListInboxPending(ctx, payerID)
}

type SentItem = store.ListSentByRequesterRow

func (s *Service) Sent(ctx context.Context, requesterID uuid.UUID) ([]SentItem, error) {
	return s.q.ListSentByRequester(ctx, requesterID)
}

type Detail struct {
	store.GetMoneyRequestRow
	Siblings []store.ListBySplitGroupRow `json:"siblings,omitempty"`
}

// DetailFor enforces ownership: only the requester or that request's payer
// may view it (404 otherwise — existence not revealed).
func (s *Service) DetailFor(ctx context.Context, userID, requestID uuid.UUID) (Detail, error) {
	req, err := s.q.GetMoneyRequest(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, ErrRequestNotFound
		}
		return Detail{}, err
	}
	if req.RequesterID != userID && req.PayerID != userID {
		return Detail{}, ErrRequestNotFound
	}
	d := Detail{GetMoneyRequestRow: req}
	if req.SplitGroupID != uuid.Nil {
		siblings, err := s.q.ListBySplitGroup(ctx, req.SplitGroupID)
		if err != nil {
			return Detail{}, err
		}
		d.Siblings = siblings
	}
	return d, nil
}

// Remind updates last_reminded_at with rate limiting of 24 hours.
func (s *Service) Remind(ctx context.Context, requestID, requesterID uuid.UUID) (time.Time, error) {
	row, err := s.q.RemindMoneyRequestGuarded(ctx, store.RemindMoneyRequestGuardedParams{
		ID:          requestID,
		RequesterID: requesterID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Row was not updated. Let's trace why using a select.
			req, getErr := s.q.GetMoneyRequest(ctx, requestID)
			if getErr != nil {
				if errors.Is(getErr, pgx.ErrNoRows) {
					return time.Time{}, ErrRequestNotFound
				}
				return time.Time{}, getErr
			}
			if req.RequesterID != requesterID {
				return time.Time{}, ErrForbidden
			}
			if req.Status != store.MoneyRequestStatusPending {
				return time.Time{}, ErrNotPayable
			}
			// If status is pending and we are the requester, but update failed,
			// it means the last_reminded_at guard (> 24 hours) blocked it.
			return time.Time{}, ErrRemindTooSoon
		}
		return time.Time{}, err
	}

	return row.LastRemindedAt.Time, nil
}

func noteText(n string) pgtype.Text {
	if n == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: n, Valid: true}
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
