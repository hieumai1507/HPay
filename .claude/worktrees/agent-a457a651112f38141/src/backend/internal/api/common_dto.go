package api

import (
	"strings"
	"time"

	"github.com/hieumai1507/hpay-backend/internal/listing"
)

// Shared pagination and query defaults
const (
	DefaultPage  = 1
	DefaultLimit = 10
	MaxLimit     = 100
	DefaultOrder = "desc"
)

// CommonQueryParams is the standardized GET query DTO shared across all API list endpoints.
type CommonQueryParams struct {
	Page   int    `form:"page,default=1" json:"page" binding:"min=1"`
	Limit  int    `form:"limit,default=10" json:"limit" binding:"min=1,max=100"`
	Name   string `form:"name" json:"name"`
	Sort   string `form:"sort" json:"sort"`
	Order  string `form:"order,default=desc" json:"order" binding:"omitempty,oneof=asc desc"`
	From   string `form:"from" json:"from"`
	To     string `form:"to" json:"to"`
	Status string `form:"status" json:"status"`
	Tab    string `form:"tab" json:"tab"`
}

// AdminPaginationQuery is an alias for CommonQueryParams
type AdminPaginationQuery = CommonQueryParams

func (q *CommonQueryParams) Normalize() {
	if q.Page < 1 {
		q.Page = DefaultPage
	}
	if q.Limit < 1 {
		q.Limit = DefaultLimit
	}
	if q.Limit > MaxLimit {
		q.Limit = MaxLimit
	}
	if q.Order == "" {
		q.Order = DefaultOrder
	}
}

// ToFilter converts the bound query string into the shared filter the service
// layer understands. Unparseable dates are dropped rather than rejected — a bad
// `from` should not 400 a list request, it just means "no lower bound".
func (q *CommonQueryParams) ToFilter() listing.Filter {
	q.Normalize()
	from, _ := q.ParseFromTime()
	to, _ := q.ParseToTime()
	return listing.Filter{
		Limit:  int32(q.Limit),
		Offset: int32(q.Offset()),
		Name:   strings.TrimSpace(q.Name),
		Order:  q.NormalizedOrder(),
		From:   from,
		To:     to,
	}
}

// TotalPages is the page count for a total row count under this page size.
func (q *CommonQueryParams) TotalPages(total int64) int {
	q.Normalize()
	if total <= 0 {
		return 1
	}
	return int((total + int64(q.Limit) - 1) / int64(q.Limit))
}

func (q *CommonQueryParams) Offset() int {
	q.Normalize()
	return (q.Page - 1) * q.Limit
}

func (q *CommonQueryParams) NormalizedOrder() string {
	if strings.ToLower(q.Order) == "asc" {
		return "asc"
	}
	return "desc"
}

func (q *CommonQueryParams) ParseFromTime() (*time.Time, error) {
	if q.From == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", q.From)
	if err != nil {
		t, err = time.Parse(time.RFC3339, q.From)
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (q *CommonQueryParams) ParseToTime() (*time.Time, error) {
	if q.To == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", q.To)
	if err != nil {
		t, err = time.Parse(time.RFC3339, q.To)
	}
	if err != nil {
		return nil, err
	}
	if len(q.To) == 10 {
		t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
	}
	return &t, nil
}
