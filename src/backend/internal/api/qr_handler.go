package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/qr"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/user"
)

type QRHandler struct {
	q     *store.Queries
	qrSvc *qr.Service
}

func NewQRHandler(pool *pgxpool.Pool, qrSvc *qr.Service) *QRHandler {
	return &QRHandler{
		q:     store.New(pool),
		qrSvc: qrSvc,
	}
}

func (h *QRHandler) Mint(c *gin.Context) {
	userID := mustUserID(c)
	u, err := h.q.GetUserProfileByID(c.Request.Context(), userID)
	if err != nil {
		respondErr(c, err)
		return
	}

	nameMasked := user.MaskName(u.FullName)
	phoneMasked := user.MaskPhone(u.Phone)

	res, err := h.qrSvc.Mint(userID, nameMasked, phoneMasked)
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

type resolveReq struct {
	Payload   string `json:"payload" binding:"required"`
	Signature string `json:"signature" binding:"required"`
}

func (h *QRHandler) Resolve(c *gin.Context) {
	var req resolveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	res, err := h.qrSvc.Resolve(req.Payload, req.Signature)
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, res)
}
