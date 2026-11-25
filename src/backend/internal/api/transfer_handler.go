package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/qr"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

type TransferHandler struct {
	svc       *transfer.Service
	walletSvc *wallet.Service
	qrSvc     *qr.Service
}

func NewTransferHandler(svc *transfer.Service, walletSvc *wallet.Service, qrSvc *qr.Service) *TransferHandler {
	return &TransferHandler{svc: svc, walletSvc: walletSvc, qrSvc: qrSvc}
}

type transferReq struct {
	ReceiverPhone string `json:"receiver_phone"`
	ReceiverToken string `json:"receiver_token"`
	Amount        int64  `json:"amount" binding:"required"`
	Note          string `json:"note" binding:"max=255"`
}

func (h *TransferHandler) Transfer(c *gin.Context) {
	var req transferReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	if req.ReceiverPhone == "" && req.ReceiverToken == "" {
		respondValidation(c, "either receiver_phone or receiver_token is required")
		return
	}
	if req.ReceiverPhone != "" && req.ReceiverToken != "" {
		respondValidation(c, "cannot provide both receiver_phone and receiver_token")
		return
	}

	var receiverID uuid.UUID
	var err error

	if req.ReceiverToken != "" {
		receiverID, err = h.qrSvc.VerifyReceiverToken(req.ReceiverToken)
		if err != nil {
			respondErr(c, err)
			return
		}
	}

	var res transfer.Result
	if req.ReceiverToken != "" {
		res, err = h.svc.TransferToUser(c.Request.Context(), mustUserID(c), receiverID, req.Amount, req.Note, c.GetString("idempotency_key"))
	} else {
		res, err = h.svc.Transfer(c.Request.Context(), mustUserID(c), req.ReceiverPhone, req.Amount, req.Note, c.GetString("idempotency_key"))
	}

	if err != nil {
		respondErr(c, err)
		return
	}
	status := http.StatusAccepted // async: client polls timeline
	if res.Duplicate {
		status = http.StatusOK
	}
	c.JSON(status, res)
}

func (h *TransferHandler) FeePreview(c *gin.Context) {
	amount, err := strconv.ParseInt(c.Query("amount"), 10, 64)
	if err != nil {
		respondValidation(c, "amount must be an integer (cents)")
		return
	}
	feeAmt, total, err := h.svc.FeePreview(c.Request.Context(), mustUserID(c), amount)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"amount": amount, "fee": feeAmt, "total": total})
}

// Timeline reuses ownership enforcement from wallet.Detail (404 if not yours).
func (h *TransferHandler) Timeline(c *gin.Context) {
	txn, err := h.walletSvc.DetailRaw(c.Request.Context(), mustUserID(c), c.Param("refCode"))
	if err != nil {
		respondErr(c, err)
		return
	}
	tl, err := h.svc.TimelineFor(c.Request.Context(), txn.ID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, tl)
}
