// Package moneyrequest implements Request Money and Split Bill (docs plans f7).
// Paying a request is a normal P2P transfer under the hood — this package only
// adds the two-phase state in front of it.
package moneyrequest

import (
	"errors"
	"fmt"
)

var ErrBadSplit = errors.New("invalid split")

// SplitEven divides total into n positive shares that sum exactly to total,
// using largest-remainder so the leftover cents spread across payers
// instead of piling onto one. Pure integer math — no float touches money.
func SplitEven(total int64, n int) ([]int64, error) {
	if n <= 0 {
		return nil, errors.New("n must be positive")
	}
	if total < int64(n) {
		// Each share must be a positive number of cents (amount > 0 per schema CHECK).
		return nil, ErrBadSplit
	}
	base := total / int64(n)
	remainder := total % int64(n)

	shares := make([]int64, n)
	for i := range shares {
		shares[i] = base
		if int64(i) < remainder {
			shares[i]++
		}
	}
	return shares, nil
}

// ValidateCustomSplit checks caller-supplied amounts sum exactly to total and
// every share is positive.
func ValidateCustomSplit(amounts []int64, total int64) error {
	if len(amounts) == 0 {
		return fmt.Errorf("no amounts given")
	}
	var sum int64
	for _, a := range amounts {
		if a <= 0 {
			return fmt.Errorf("each split amount must be positive")
		}
		sum += a
	}
	if sum != total {
		return fmt.Errorf("split amounts sum to %d, want %d", sum, total)
	}
	return nil
}
