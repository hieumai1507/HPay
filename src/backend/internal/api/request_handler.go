package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/moneyrequest"
)

type RequestHandler struct {
	svc *moneyrequest.Service
}

func NewRequestHandler(svc *moneyrequest.Service) *RequestHandler { return &RequestHandler{svc: svc} }

type createRequestReq struct {
	PayerPhone string `json:"payer_phone" binding:"required"`
	Amount     int64  `json:"amount" binding:"required"`
	Note       string `json:"note" binding:"max=255"`
}

func (h *RequestHandler) Create(c *gin.Context) {
	var req createRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.svc.Create(c.Request.Context(), mustUserID(c), req.PayerPhone, req.Amount, req.Note)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

type createSplitReq struct {
	PayerPhones []string `json:"payer_phones" binding:"required,min=1"`
	TotalAmount int64    `json:"total_amount" binding:"required"`
	Note        string   `json:"note" binding:"max=255"`
	Amounts     []int64  `json:"amounts"`
}

func (h *RequestHandler) CreateSplit(c *gin.Context) {
	var req createSplitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.svc.CreateSplit(c.Request.Context(), mustUserID(c), req.PayerPhones, req.TotalAmount, req.Note, req.Amounts)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *RequestHandler) Inbox(c *gin.Context) {
	rows, err := h.svc.Inbox(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": rows})
}

func (h *RequestHandler) Sent(c *gin.Context) {
	rows, err := h.svc.Sent(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": rows})
}

func (h *RequestHandler) Detail(c *gin.Context) {
	id, ok := requestIDParam(c)
	if !ok {
		return
	}
	d, err := h.svc.DetailFor(c.Request.Context(), mustUserID(c), id)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h *RequestHandler) Decline(c *gin.Context) {
	id, ok := requestIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.Decline(c.Request.Context(), mustUserID(c), id); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "declined"})
}

func (h *RequestHandler) Cancel(c *gin.Context) {
	id, ok := requestIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), mustUserID(c), id); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "canceled"})
}

func (h *RequestHandler) Remind(c *gin.Context) {
	id, ok := requestIDParam(c)
	if !ok {
		return
	}
	lastRemindedAt, err := h.svc.Remind(c.Request.Context(), id, mustUserID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"last_reminded_at": lastRemindedAt})
}

func requestIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid money request id")
		return uuid.Nil, false
	}
	return id, true
}
