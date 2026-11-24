package fraud

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/ai"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

// ErrBlocked is the terminal error returned when rules block a transaction —
// the workflow's on_failure=fail path turns it into a failed transaction.
var ErrBlocked = errors.New("transaction blocked by fraud rules")

// RegisterHandler replaces the F5 FraudStub with the real rule engine.
func RegisterHandler(reg *workflow.Registry, pool *pgxpool.Pool, log *slog.Logger) {
	reg.Register(&handler{pool: pool, q: store.New(pool), log: log})
}

type handler struct {
	pool *pgxpool.Pool
	q    *store.Queries
	log  *slog.Logger
	// advisorOverride replaces the advisor chosen from the AI settings. Tests only.
	advisorOverride ai.Advisor
	// advisoryDone, when set, is closed-over by tests to learn that the advisory goroutine finished.
	advisoryDone func()
}

func (*handler) Name() string { return "fraud" }

func (h *handler) Execute(ctx context.Context, in workflow.StepInput) (workflow.StepOutput, error) {
	if in.Compensating {
		return workflow.StepOutput{}, nil
	}
	txn, err := h.q.GetTransactionByWorkflow(ctx, in.TransactionID)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}

	// Gather rule inputs (read-only history queries).
	velocity, err := h.q.CountSenderTxnsSince(ctx, store.CountSenderTxnsSinceParams{
		InitiatedBy: txn.InitiatedBy, CreatedAt: pgts(time.Now().Add(-5 * time.Minute)),
	})
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	avg, err := h.q.AvgSenderAmount30d(ctx, txn.InitiatedBy)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	seen, err := h.q.HasSentToReceiver(ctx, store.HasSentToReceiverParams{
		InitiatedBy: txn.InitiatedBy, ReceiverWalletID: txn.ReceiverWalletID,
	})
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}

	signals, score := Evaluate(Input{
		Amount: txn.Amount, Fee: txn.Fee, Avg30dAmount: avg,
		TxnsInLast5Min: int(velocity), IsNewReceiver: !seen,
		HourOfDay: time.Now().Hour(),
	})
	decision := Decide(score)

	if decision == DecisionPass {
		return workflow.StepOutput{Context: map[string]any{"fraud_score": score, "fraud_verdict": "safe"}}, nil
	}

	// Flag or block: record a fraud_flags row with the deterministic evidence.
	reasons, _ := json.Marshal(map[string]any{"signals": signals})
	flagID, created, err := h.insertFlagOnce(ctx, store.InsertFraudFlagParams{
		TransactionID: txn.ID, RuleCode: topRuleCode(signals), Score: int32(score), Reasons: reasons,
	})
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}

	// LLM advisory is fire-and-forget: the FRAUD_CHECK step has a 3s budget but
	// the model takes seconds — and the model must NEVER affect the decision
	// (docs §9). Kick it off on a detached context and merge the text later.
	// A redelivered step reuses the flag and does not pay for a second advisory.
	if created {
		h.enqueueAdvisory(flagID, txn, signals, score)
	}

	if decision == DecisionBlock {
		h.log.Warn("transaction blocked by fraud rules", "txn", txn.ID, "score", score)
		return workflow.StepOutput{}, ErrBlocked // terminal -> on_failure=fail
	}
	// Grey zone: flag but let the money proceed.
	return workflow.StepOutput{Context: map[string]any{"fraud_score": score, "fraud_verdict": "flagged", "fraud_flag_id": flagID.String()}}, nil
}

// insertFlagOnce writes the flag for a transaction at most once (G19, BUG-0019). The step
// record is committed after this write, so a crash or timeout in between makes the broker
// redeliver the step; a plain INSERT would then leave two open flags for one transaction.
// A transaction-scoped advisory lock serialises concurrent redeliveries on the same
// transaction (fraud_flags has no unique key on transaction_id and migrations are frozen
// for this change). created is false when an earlier run's flag was reused.
func (h *handler) insertFlagOnce(ctx context.Context, p store.InsertFraudFlagParams) (id uuid.UUID, created bool, err error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('fraud_flag:' || $1::text, 0))`, p.TransactionID); err != nil {
		return uuid.Nil, false, err
	}
	qtx := h.q.WithTx(tx)
	id, err = qtx.GetFraudFlagIDByTransaction(ctx, p.TransactionID)
	switch {
	case err == nil:
		return id, false, tx.Commit(ctx)
	case !errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, false, err
	}
	if id, err = qtx.InsertFraudFlag(ctx, p); err != nil {
		return uuid.Nil, false, err
	}
	return id, true, tx.Commit(ctx)
}

// enqueueAdvisory runs the LLM off the step's critical path. Whatever happens here (no
// setting, inactive key, provider error, empty text, a failed write) the decision already
// taken from the rules stands: the advisory is only ever an extra note on the flag.
func (h *handler) enqueueAdvisory(flagID uuid.UUID, txn store.GetTransactionByWorkflowRow, signals []Signal, score int) {
	go func() {
		if h.advisoryDone != nil {
			defer h.advisoryDone()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		advisor := h.advisorOverride
		if advisor == nil {
			setting, err := h.q.GetActiveAISetting(ctx, "fraud_advising")
			if err != nil {
				h.log.Warn("fetch fraud advising AI setting failed", "flag", flagID, "err", err)
				return
			}
			switch setting.Provider {
			case "anthropic", "openai":
				if !setting.KeyActive.Valid || !setting.KeyActive.Bool {
					h.log.Debug("fraud advising key is inactive", "flag", flagID)
					return
				}
				if setting.Provider == "anthropic" {
					advisor = ai.NewClaudeAdvisor(setting.ApiKey.String)
				} else {
					advisor = ai.NewOpenAIAdvisor(setting.ApiKey.String, setting.ModelName)
				}
			default: // "mock" and anything unknown
				advisor = ai.MockAdvisor{}
			}
		}

		evidence, _ := json.Marshal(signals)
		user := "Transaction ref " + txn.RefCode + ", risk score " + itoa(score) +
			". Signals: " + string(evidence) +
			". Explain in English, briefly, why this transaction looks suspicious, for the reviewing analyst."
		text, err := advisor.Complete(ctx,
			"You are a fraud analysis assistant. Only explain; never decide to block or release.", user)
		if err != nil || text == "" {
			h.log.Debug("fraud advisory unavailable", "flag", flagID, "err", err)
			return // no advice is fine: the rules already decided
		}
		merged, _ := json.Marshal(map[string]any{"signals": signals, "advisory": text})
		if err := h.q.AppendFraudFlagReasons(context.Background(), store.AppendFraudFlagReasonsParams{
			ID: flagID, Reasons: merged,
		}); err != nil {
			h.log.Warn("append fraud advisory failed", "flag", flagID, "err", err)
		}
	}()
}
func topRuleCode(signals []Signal) string {
	top := "MULTI"
	best := -1
	for _, s := range signals {
		if s.Points > best {
			best, top = s.Points, s.Code
		}
	}
	return top
}
