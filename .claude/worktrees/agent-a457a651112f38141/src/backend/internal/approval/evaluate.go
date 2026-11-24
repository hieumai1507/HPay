// Package approval implements the generalized maker-checker engine: policy
// snapshots, multi-tier signatures, OTP-gated decisions, and workflow resume
// (docs plans f6). It is subject-agnostic — executors per subject_type run
// the effect when a request finalizes.
package approval

import (
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// Tier is one signature level of a policy (docs 00 §3.4).
type Tier struct {
	Level             int `json:"level"`
	RequiredApprovals int `json:"required_approvals"`
}

// PolicySnapshot is frozen into approval_requests at creation time so a later
// policy change never affects an in-flight request (E09).
type PolicySnapshot struct {
	PolicyID uuid.UUID `json:"policy_id"`
	Name     string    `json:"name"`
	Version  int       `json:"version"`
	Tiers    []Tier    `json:"tiers"`
}

// ActionView is the pure view of one recorded checker action.
type ActionView struct {
	Level   int
	Approve bool
}

type DecisionStatus string

const (
	DecisionPending  DecisionStatus = "pending"
	DecisionApproved DecisionStatus = "approved"
	DecisionRejected DecisionStatus = "rejected"
)

// Decision is the outcome of evaluating a snapshot against recorded actions.
type Decision struct {
	Status       DecisionStatus
	CurrentLevel int // meaningful when Status == DecisionPending
}

// Evaluate is the Go implementation of the approval policy evaluation (see docs/domain/approval-maker-checker.md):
// any reject is final; otherwise tiers must fill up in ascending level order.
// Pure function — the caller holds the FOR UPDATE lock and passes consistent
// data.
func Evaluate(snap PolicySnapshot, actions []ActionView) Decision {
	for _, a := range actions {
		if !a.Approve {
			return Decision{Status: DecisionRejected}
		}
	}
	tiers := sortedTiers(snap.Tiers)
	counts := map[int]int{}
	for _, a := range actions {
		counts[a.Level]++
	}
	for _, t := range tiers {
		if counts[t.Level] < t.RequiredApprovals {
			return Decision{Status: DecisionPending, CurrentLevel: t.Level}
		}
	}
	return Decision{Status: DecisionApproved}
}

var ErrBadTiers = errors.New("invalid policy tiers")

// ValidateTiers rejects malformed tier configurations before they are stored.
func ValidateTiers(tiers []Tier) error {
	if len(tiers) == 0 {
		return fmt.Errorf("%w: at least one tier required", ErrBadTiers)
	}
	seen := map[int]bool{}
	for _, t := range tiers {
		if t.Level < 1 {
			return fmt.Errorf("%w: level must be >= 1", ErrBadTiers)
		}
		if t.RequiredApprovals < 1 {
			return fmt.Errorf("%w: required_approvals must be >= 1", ErrBadTiers)
		}
		if seen[t.Level] {
			return fmt.Errorf("%w: duplicate level %d", ErrBadTiers, t.Level)
		}
		seen[t.Level] = true
	}
	return nil
}

func sortedTiers(tiers []Tier) []Tier {
	out := make([]Tier, len(tiers))
	copy(out, tiers)
	sort.Slice(out, func(i, j int) bool { return out[i].Level < out[j].Level })
	return out
}
