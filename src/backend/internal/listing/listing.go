// Package listing carries the shared paging/filter arguments for admin list
// endpoints from the API layer down to the store, without services having to
// import the HTTP layer (or repeat six positional arguments each).
package listing

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Filter is the normalized form of the admin query string. A zero value means
// "first page, no filters", so callers can pass listing.Filter{} safely.
type Filter struct {
	Limit  int32
	Offset int32
	// Name is a free-text needle (name / code / phone / ref_code depending on
	// the endpoint). Empty means no text filter — NOT a '%%' match.
	Name  string
	Order string // "asc" | "desc"; anything else is treated as the query default
	From  *time.Time
	To    *time.Time
}

// Normalize fills in safe defaults so a partially-built Filter cannot produce
// LIMIT 0 (which would silently return nothing).
func (f Filter) Normalize() Filter {
	if f.Limit <= 0 {
		f.Limit = 10
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return f
}

// Text renders the search needle as a nullable param: absent when empty so the
// `narg IS NULL OR ...` branch matches every row.
func (f Filter) Text() pgtype.Text {
	if f.Name == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: f.Name, Valid: true}
}

func (f Filter) FromTS() pgtype.Timestamptz {
	if f.From == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *f.From, Valid: true}
}

func (f Filter) ToTS() pgtype.Timestamptz {
	if f.To == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *f.To, Valid: true}
}

// SortOrder is the non-null string the ORDER BY CASE expressions compare
// against; it never returns an empty string.
func (f Filter) SortOrder() string {
	if f.Order == "asc" {
		return "asc"
	}
	return "desc"
}
