package api

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

// MockBankHandler receives simulated bank notifications (F10 E15). Mounted
// under /internal/ — never exposed publicly; a deploy must block the prefix at
// the proxy. Auth is a shared secret header, not a user JWT.
type MockBankHandler struct {
	svc    *wallet.Service
	secret string
}

func NewMockBankHandler(svc *wallet.Service, secret string) *MockBankHandler {
	return &MockBankHandler{svc: svc, secret: secret}
}

type bankCallbackReq struct {
	RefCode string `json:"ref_code" binding:"required"`
	Amount  int64  `json:"amount" binding:"required"`
	BankRef string `json:"bank_ref"`
}

func (h *MockBankHandler) Callback(c *gin.Context) {
	got := c.GetHeader("X-Mock-Bank-Secret")
	if subtle.ConstantTimeCompare([]byte(got), []byte(h.secret)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_INVALID_CALLBACK_SECRET",
			"message": "invalid callback secret", "request_id": c.GetString("request_id")})
		return
	}
	var req bankCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.svc.BankCallback(c.Request.Context(), req.RefCode, req.Amount, req.BankRef)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
