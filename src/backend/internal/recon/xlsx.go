package recon

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/xuri/excelize/v2"
)

// buildReport writes an xlsx of the run into storage and returns its key
// (empty on failure — the report is a convenience, not a correctness gate).
func (s *Service) buildReport(ctx context.Context, day time.Time, items []classifiedItem) string {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Reconciliation"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"Result", "Transaction", "Statement", "Detail"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	for r, it := range items {
		row := r + 2
		txn := ""
		if it.txnID.String() != "00000000-0000-0000-0000-000000000000" {
			txn = it.txnID.String()
		}
		stmt := ""
		if it.statementID.Valid {
			stmt = itoa64(it.statementID.Int64)
		}
		detail, _ := json.Marshal(it.detail)
		vals := []any{string(it.result), txn, stmt, string(detail)}
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return ""
	}
	key := "recon/" + day.Format("2006-01-02") + ".xlsx"
	if err := s.files.Put(ctx, key, buf.Bytes()); err != nil {
		return ""
	}
	return key
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
