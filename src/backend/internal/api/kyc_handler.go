package api

import (
	"io"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hieumai1507/hpay-backend/internal/kyc"
)

type KYCHandler struct {
	svc *kyc.Service
}

func NewKYCHandler(svc *kyc.Service) *KYCHandler { return &KYCHandler{svc: svc} }

// Submit takes multipart fields id_card + selfie (jpeg/png, ≤5MB each).
func (h *KYCHandler) Submit(c *gin.Context) {
	idCard, ok := readImagePart(c, "id_card")
	if !ok {
		return
	}
	selfie, ok := readImagePart(c, "selfie")
	if !ok {
		return
	}
	res, err := h.svc.Submit(c.Request.Context(), mustUserID(c), idCard, selfie)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, res) // verdict lands async — poll /kyc/status
}

func (h *KYCHandler) Status(c *gin.Context) {
	res, err := h.svc.Status(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func readImagePart(c *gin.Context, field string) ([]byte, bool) {
	fh, err := c.FormFile(field)
	if err != nil {
		respondValidation(c, "multipart field "+field+" is required")
		return nil, false
	}
	f, err := fh.Open()
	if err != nil {
		respondValidation(c, "cannot read "+field)
		return nil, false
	}
	defer func(f multipart.File) { _ = f.Close() }(f)
	data, err := io.ReadAll(io.LimitReader(f, 6<<20)) // hard cap slightly above service limit
	if err != nil {
		respondValidation(c, "cannot read "+field)
		return nil, false
	}
	return data, true
}
