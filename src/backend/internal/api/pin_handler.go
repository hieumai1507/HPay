package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hieumai1507/hpay-backend/internal/auth"
)

// PINHandler sets and changes the transaction PIN (ADR-0008, ADR-0014).
// The PIN is never logged and never returned.
type PINHandler struct{ svc *auth.Service }

func NewPINHandler(svc *auth.Service) *PINHandler { return &PINHandler{svc: svc} }

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
