package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

type WalletHandler struct {
	svc *wallet.Service
}

func NewWalletHandler(svc *wallet.Service) *WalletHandler { return &WalletHandler{svc: svc} }

func (h *WalletHandler) List(c *gin.Context) {
	uid := mustUserID(c)
	rows, err := h.svc.ListWallets(c.Request.Context(), uid)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"wallets": rows})
}

type topupReq struct {
	Amount int64 `json:"amount" binding:"required"`
}

func (h *WalletHandler) Topup(c *gin.Context) {
	var req topupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.svc.Topup(c.Request.Context(), mustUserID(c), req.Amount, c.GetString("idempotency_key"))
	if err != nil {
		respondErr(c, err)
		return
	}
	status := http.StatusCreated
	if res.Duplicate {
		status = http.StatusOK // E01: same key -> original transaction
	}
	c.JSON(status, res)
}

func (h *WalletHandler) History(c *gin.Context) {
	before := time.Now().Add(time.Minute)
	beforeID := uuid.Max
	if cur := c.Query("cursor"); cur != "" {
		if t, err := time.Parse(time.RFC3339Nano, cur); err == nil {
			before = t
		}
		if id, err := uuid.Parse(c.Query("cursor_id")); err == nil {
			beforeID = id
		}
	}
	rows, err := h.svc.History(c.Request.Context(), mustUserID(c), before, beforeID, 20)
	if err != nil {
		respondErr(c, err)
		return
	}
	next := ""
	nextID := ""
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		next = last.CreatedAt.Format(time.RFC3339Nano)
		nextID = last.ID.String()
	}
	c.JSON(http.StatusOK, gin.H{"transactions": rows, "next_cursor": next, "next_cursor_id": nextID})
}

func (h *WalletHandler) Detail(c *gin.Context) {
	txn, err := h.svc.Detail(c.Request.Context(), mustUserID(c), c.Param("refCode"))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, txn)
}

func mustUserID(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("user_id"))
	return id
}
