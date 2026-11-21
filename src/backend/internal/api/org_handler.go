package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/org"
)

type OrgHandler struct {
	svc *org.Service
}

func NewOrgHandler(svc *org.Service) *OrgHandler { return &OrgHandler{svc: svc} }

type createOrgReq struct {
	Name string `json:"name" binding:"required,min=2,max=200"`
}

func (h *OrgHandler) Create(c *gin.Context) {
	var req createOrgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.svc.CreateOrg(c.Request.Context(), mustUserID(c), req.Name)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *OrgHandler) ListMyOrgs(c *gin.Context) {
	res, err := h.svc.ListUserOrganizations(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"organizations": res})
}

type addMemberReq struct {
	Phone        string `json:"phone" binding:"required"`
	Role         string `json:"role" binding:"required,oneof=admin maker checker"`
	CheckerLevel int    `json:"checker_level"`
}

func (h *OrgHandler) AddMember(c *gin.Context) {
	orgID, ok := orgIDParam(c)
	if !ok {
		return
	}
	var req addMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	if err := h.svc.AddMember(c.Request.Context(), mustUserID(c), orgID, req.Phone, req.Role, req.CheckerLevel); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "member_added"})
}

type createPolicyReq struct {
	Name      string          `json:"name" binding:"required,max=100"`
	MinAmount int64           `json:"min_amount"`
	MaxAmount *int64          `json:"max_amount"`
	Tiers     []approval.Tier `json:"tiers" binding:"required"`
}

func (h *OrgHandler) CreatePolicy(c *gin.Context) {
	orgID, ok := orgIDParam(c)
	if !ok {
		return
	}
	var req createPolicyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.svc.CreatePolicy(c.Request.Context(), mustUserID(c), orgID, req.Name, req.MinAmount, req.MaxAmount, req.Tiers)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

type disburseReq struct {
	Phone  string `json:"phone" binding:"required"`
	Amount int64  `json:"amount" binding:"required,gt=0"`
	Note   string `json:"note"`
}

// Disburse is the maker action (F6): creates an org disbursement transaction
// and starts the approval-gated workflow. See org.Service.CreateDisbursement.
func (h *OrgHandler) Disburse(c *gin.Context) {
	orgID, ok := orgIDParam(c)
	if !ok {
		return
	}
	var req disburseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.svc.CreateDisbursement(c.Request.Context(), mustUserID(c), orgID, req.Phone, req.Amount, req.Note, c.GetString("idempotency_key"))
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

func orgIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid organization id")
		return uuid.Nil, false
	}
	return id, true
}
