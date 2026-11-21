package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
)

type TransactionEngineHandler struct {
	transferSvc *transfer.Service
	cfg         config.Config
}

func NewTransactionEngineHandler(transferSvc *transfer.Service, cfg config.Config) *TransactionEngineHandler {
	return &TransactionEngineHandler{
		transferSvc: transferSvc,
		cfg:         cfg,
	}
}

type engineRequestReq struct {
	ServiceCode   string                 `json:"service_code" binding:"required"`
	Amount        int64                  `json:"amount" binding:"required,gt=0"`
	ReceiverPhone string                 `json:"receiver_phone"`
	ReceiverID    *uuid.UUID             `json:"receiver_id"`
	Note          string                 `json:"note"`
	ExtraParams   map[string]interface{} `json:"extra_params"`
}

func (h *TransactionEngineHandler) RequestTransaction(c *gin.Context) {
	var req engineRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	senderUserID, _ := uuid.Parse(c.GetString("user_id"))

	// idempotency_key comes from the Idempotency-Key header (set into the
	// context by RequireIdempotencyKey) per openapi.yaml — not a body field.
	result, err := h.transferSvc.ExecuteEngineRequest(c.Request.Context(), transfer.EngineRequestInput{
		ServiceCode:   req.ServiceCode,
		SenderUserID:  senderUserID,
		ReceiverPhone: req.ReceiverPhone,
		ReceiverID:    req.ReceiverID,
		Amount:        req.Amount,
		Note:          req.Note,
		Idempotency:   c.GetString("idempotency_key"),
		ExtraParams:   req.ExtraParams,
	})

	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

type engineConfirmReq struct {
	TransactionID uuid.UUID `json:"transaction_id" binding:"required"`
	PIN           string    `json:"pin"`
	OTP           string    `json:"otp"`
}

func (h *TransactionEngineHandler) ConfirmTransaction(c *gin.Context) {
	var req engineConfirmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	senderUserID, _ := uuid.Parse(c.GetString("user_id"))

	result, err := h.transferSvc.ExecuteEngineConfirm(c.Request.Context(), transfer.EngineConfirmInput{
		TransactionID: req.TransactionID,
		SenderUserID:  senderUserID,
		PIN:           req.PIN,
		OTP:           req.OTP,
	})

	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

type engineVerifyReq struct {
	TransactionID uuid.UUID `json:"transaction_id" binding:"required"`
	TwoFACode     string    `json:"two_fa_code" binding:"required"`
}

func (h *TransactionEngineHandler) VerifyTransaction(c *gin.Context) {
	var req engineVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	senderUserID, _ := uuid.Parse(c.GetString("user_id"))

	result, err := h.transferSvc.ExecuteEngineVerify(c.Request.Context(), req.TransactionID, senderUserID, req.TwoFACode)
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

type engineChallengeReq struct {
	TransactionID uuid.UUID `json:"transaction_id" binding:"required"`
}

// ChallengeTransaction issues the OTP for a transaction in the otp band. The code
// is returned only when APP_ENV=dev; other environments deliver it out of band.
func (h *TransactionEngineHandler) ChallengeTransaction(c *gin.Context) {
	var req engineChallengeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	code, err := h.transferSvc.ExecuteEngineChallenge(c.Request.Context(), mustUserID(c), req.TransactionID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, challengeBody(h.cfg, code))
}
