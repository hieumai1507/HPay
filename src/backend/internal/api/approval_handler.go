package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
)

type ApprovalHandler struct {
	svc *approval.Service
	cfg config.Config
}

func NewApprovalHandler(svc *approval.Service, cfg config.Config) *ApprovalHandler {
	return &ApprovalHandler{svc: svc, cfg: cfg}
}

// Inbox is the checker's "My Tasks" queue (pending only in F6).
func (h *ApprovalHandler) Inbox(c *gin.Context) {
	if st := c.Query("status"); st != "" && st != "pending" {
		respondValidation(c, "only status=pending is supported")
		return
	}
	rows, err := h.svc.Inbox(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"approvals": rows})
}

// Challenge checks eligibility then issues the signing OTP. The code is only
// exposed in the response when ENV=dev (docs 01 §3.1) — prod sends out-of-band.
func (h *ApprovalHandler) Challenge(c *gin.Context) {
	reqID, ok := approvalIDParam(c)
	if !ok {
		return
	}
	code, err := h.svc.Challenge(c.Request.Context(), mustUserID(c), reqID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, challengeBody(h.cfg, code))
}

type decideReq struct {
	Action string `json:"action" binding:"required,oneof=approve reject"`
	OTP    string `json:"otp" binding:"required,len=6"`
}

func (h *ApprovalHandler) Decide(c *gin.Context) {
	reqID, ok := approvalIDParam(c)
	if !ok {
		return
	}
	var req decideReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.svc.Decide(c.Request.Context(), mustUserID(c), reqID, req.Action == "approve", req.OTP)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// Detail returns one request's full status + signature history for the
// maker (created_by) or any member of its org — see approval.Service.Detail.
func (h *ApprovalHandler) Detail(c *gin.Context) {
	reqID, ok := approvalIDParam(c)
	if !ok {
		return
	}
	d, err := h.svc.Detail(c.Request.Context(), mustUserID(c), reqID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

// DetailByRefCode lets the maker's own client resolve a just-created
// disbursement's approval request without knowing its id yet (the
// approval_request row is only created once the engine pauses at
// WAIT_APPROVAL, after CreateDisbursement has already returned).
func (h *ApprovalHandler) DetailByRefCode(c *gin.Context) {
	refCode := c.Param("ref_code")
	d, err := h.svc.DetailByRefCode(c.Request.Context(), mustUserID(c), refCode)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

// Cancel lets the maker withdraw their own still-pending request (mockup
// "Cancel order", frame 19). Only the request's own maker may cancel it.
func (h *ApprovalHandler) Cancel(c *gin.Context) {
	reqID, ok := approvalIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), mustUserID(c), reqID); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

func approvalIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid approval request id")
		return uuid.Nil, false
	}
	return id, true
}
