package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
)

// PINHandler sets and changes the transaction PIN (ADR-0008, ADR-0014).
// The PIN is never logged and never returned.
type PINHandler struct {
	svc *auth.Service
	cfg config.Config
}

func NewPINHandler(svc *auth.Service, cfg config.Config) *PINHandler {
	return &PINHandler{svc: svc, cfg: cfg}
}

type setPINReq struct {
	Password string `json:"password" binding:"required"`
	NewPIN   string `json:"new_pin" binding:"required"`
}

// Set creates the first PIN. It needs the account password (step-up).
func (h *PINHandler) Set(c *gin.Context) {
	var req setPINReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, "password and new_pin are required")
		return
	}
	if err := h.svc.SetPIN(c.Request.Context(), mustUserID(c), req.Password, req.NewPIN); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "pin_set"})
}

type changePINReq struct {
	CurrentPIN string `json:"current_pin" binding:"required"`
	NewPIN     string `json:"new_pin" binding:"required"`
}

// Change replaces the PIN. It needs the current PIN, which uses the same
// attempt counter and lockout as a payment confirmation.
func (h *PINHandler) Change(c *gin.Context) {
	var req changePINReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, "current_pin and new_pin are required")
		return
	}
	if err := h.svc.ChangePIN(c.Request.Context(), mustUserID(c), req.CurrentPIN, req.NewPIN); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "pin_changed"})
}

type resetRequestReq struct {
	Password string `json:"password" binding:"required"`
}

// ResetRequest starts a forgotten-PIN reset: password step-up, then a code through the
// delivery port (ADR-0016). The code is in the response only in dev.
func (h *PINHandler) ResetRequest(c *gin.Context) {
	var req resetRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, "password is required")
		return
	}
	out, err := h.svc.RequestPINReset(c.Request.Context(), mustUserID(c), req.Password)
	if err != nil {
		respondErr(c, err)
		return
	}
	body := gin.H{"status": "code_sent", "expires_in": out.ExpiresIn}
	if h.cfg.IsDev() && out.DevCode != "" {
		body["otp_dev"] = out.DevCode
	}
	c.JSON(http.StatusOK, body)
}

type resetConfirmReq struct {
	Code   string `json:"code" binding:"required"`
	NewPIN string `json:"new_pin" binding:"required"`
}

// ResetConfirm checks the code and sets the new PIN.
func (h *PINHandler) ResetConfirm(c *gin.Context) {
	var req resetConfirmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, "code and new_pin are required")
		return
	}
	if err := h.svc.ConfirmPINReset(c.Request.Context(), mustUserID(c), req.Code, req.NewPIN); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "pin_reset"})
}
