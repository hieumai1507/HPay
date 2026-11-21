// Package fraud runs deterministic rules on a transaction inside the
// FRAUD_CHECK step and, for grey-zone scores, attaches an LLM advisory
// AFTER the decision (docs 00 §8.2). The LLM never decides — the block/flag
// call is pure integer rules; the model only explains.
package fraud

import "fmt"

// Thresholds (docs 00 §8.2: grey zone 40-70; above blocks).
const (
	FlagAt   = 40 // >= this: raise a flag
	BlockAt  = 70 // > this: block the transaction
	MaxScore = 100
)

// Signal is one fired rule: its code, the points it adds, and human evidence.
type Signal struct {
	Code     string `json:"code"`
	Points   int    `json:"points"`
	Evidence string `json:"evidence"`
}

// Input is the pure view a rule set needs — no DB, no clock, fully testable.
type Input struct {
	Amount         int64
	Fee            int64
	Avg30dAmount   int64 // 0 when the sender has no history
	TxnsInLast5Min int   // includes the current one
	IsNewReceiver  bool
	HourOfDay      int // 0-23, local
}

// Evaluate runs every rule and returns the fired signals plus the total score
// (capped at 100). Pure and deterministic.
func Evaluate(in Input) ([]Signal, int) {
	var signals []Signal

	// VELOCITY_5MIN: many transfers in a short window.
	if in.TxnsInLast5Min >= 4 {
		signals = append(signals, Signal{
			Code: "VELOCITY_5MIN", Points: 30,
			Evidence: fmt.Sprintf("%d giao dịch trong 5 phút", in.TxnsInLast5Min),
		})
	}

	// AMOUNT_SPIKE: far above the sender's 30-day average.
	if in.Avg30dAmount > 0 && in.Amount >= in.Avg30dAmount*5 {
		signals = append(signals, Signal{
			Code: "AMOUNT_SPIKE", Points: 35,
			Evidence: fmt.Sprintf("số tiền %d gấp %d lần trung bình 30 ngày (%d)",
				in.Amount, in.Amount/max64(in.Avg30dAmount, 1), in.Avg30dAmount),
		})
	}

	// ODD_HOURS: 0:00-4:59.
	if in.HourOfDay >= 0 && in.HourOfDay < 5 {
		signals = append(signals, Signal{
			Code: "ODD_HOURS", Points: 20,
			Evidence: fmt.Sprintf("giao dịch lúc %02d giờ", in.HourOfDay),
		})
	}

	// NEW_RECEIVER: first time paying this receiver.
	if in.IsNewReceiver {
		signals = append(signals, Signal{
			Code: "NEW_RECEIVER", Points: 20,
			Evidence: "người nhận chưa từng nhận tiền từ tài khoản này",
		})
	}

	score := 0
	for _, s := range signals {
		score += s.Points
	}
	if score > MaxScore {
		score = MaxScore
	}
	return signals, score
}

// Decision names the outcome for a score.
type Decision string

const (
	DecisionPass  Decision = "pass"
	DecisionFlag  Decision = "flag"
	DecisionBlock Decision = "block"
)

func Decide(score int) Decision {
	switch {
	case score > BlockAt:
		return DecisionBlock
	case score >= FlagAt:
		return DecisionFlag
	default:
		return DecisionPass
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
