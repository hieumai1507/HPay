package approval

import (
	"errors"
	"fmt"
)

// ManualError is returned by an Executor when the approved effect cannot be
// applied for a business reason that a person must resolve (ADR-0013), for
// example a reversal whose receiver no longer holds the money. The approval
// service then discards the executor's partial work (savepoint), keeps the
// approver's decision, parks the request as needs_manual and writes an audit
// row with Action and Resource. It is NOT used for bugs or infrastructure
// errors: those still fail the whole decision.
type ManualError struct {
	Action   string // audit action, for example "reversal.needs_manual"
	Resource string // audit resource, for example "transaction MP123"
	Reason   string // human readable, stored as the audit reason
}

func (e *ManualError) Error() string {
	return fmt.Sprintf("needs manual resolution: %s (%s)", e.Reason, e.Resource)
}

// NeedsManual builds a ManualError.
func NeedsManual(action, resource, reason string) error {
	return &ManualError{Action: action, Resource: resource, Reason: reason}
}

// asManual reports whether err is (or wraps) a ManualError.
func asManual(err error) (*ManualError, bool) {
	var me *ManualError
	if errors.As(err, &me) {
		return me, true
	}
	return nil, false
}
