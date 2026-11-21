package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/fraud"
	"github.com/hieumai1507/hpay-backend/internal/product"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// --- fraud flags --------------------------------------------------------------

// fraudFlagResponse mirrors fraud.FlagItem but re-types Reasons as
// json.RawMessage — fraud.FlagItem.Reasons is a raw []byte (the jsonb column
// scanned unparsed), and encoding/json marshals a bare []byte as a base64
// string, not as embedded JSON. Serializing fraud.FlagItem directly sent the
// admin console an unreadable base64 blob where it expected an array, which
// crashed the whole Fraud Monitor page.
type fraudFlagResponse struct {
	ID            uuid.UUID          `json:"id"`
	TransactionID uuid.UUID          `json:"transaction_id"`
	RefCode       string             `json:"ref_code"`
	RuleCode      string             `json:"rule_code"`
	Score         int32              `json:"score"`
	Reasons       json.RawMessage    `json:"reasons"`
	Status        store.FlagStatus   `json:"status"`
	CreatedAt     pgtype.Timestamptz `json:"created_at"`
}

func toFraudFlagResponse(items []fraud.FlagItem) []fraudFlagResponse {
	out := make([]fraudFlagResponse, len(items))
	for i, f := range items {
		out[i] = fraudFlagResponse{
			ID: f.ID, TransactionID: f.TransactionID, RefCode: f.RefCode,
			RuleCode: f.RuleCode, Score: f.Score, Reasons: json.RawMessage(f.Reasons),
			Status: f.Status, CreatedAt: f.CreatedAt,
		}
	}
	return out
}

func (h *AdminHandler) FraudFlags(c *gin.Context) {
	var query AdminPaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query.Normalize()
	}
	st := query.Status
	if st == "" {
		st = c.Query("status")
	}
	rows, total, err := h.fraudSvc.List(c.Request.Context(), st, query.ToFilter())
	if err != nil {
		respondValidation(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"flags":       toFraudFlagResponse(rows),
		"total":       total,
		"page":        query.Page,
		"limit":       query.Limit,
		"total_pages": query.TotalPages(total),
	})
}

type resolveFlagReq struct {
	Action string `json:"action" binding:"required,oneof=clear confirm_fraud"`
	Reason string `json:"reason" binding:"max=255"`
}

func (h *AdminHandler) ResolveFraudFlag(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid fraud flag id")
		return
	}
	var req resolveFlagReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	if err := h.fraudSvc.Resolve(c.Request.Context(), mustAdminID(c), id, req.Action == "confirm_fraud", req.Reason); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "resolved"})
}

// --- reversal + admin approvals ----------------------------------------------

type createReversalReq struct {
	RefCode string `json:"ref_code" binding:"required"`
	Reason  string `json:"reason" binding:"max=255"`
}

func (h *AdminHandler) CreateReversal(c *gin.Context) {
	var req createReversalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.revSvc.CreateRequest(c.Request.Context(), mustAdminID(c), req.RefCode, req.Reason)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *AdminHandler) AdminApprovals(c *gin.Context) {
	var query AdminPaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query.Normalize()
	}
	st := query.Status
	if st == "" {
		st = c.Query("status")
	}
	if st != "" && st != "pending" {
		respondValidation(c, "only status=pending is supported")
		return
	}
	rows, total, err := h.aprSvc.AdminInbox(c.Request.Context(), query.ToFilter())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"approvals":   rows,
		"total":       total,
		"page":        query.Page,
		"limit":       query.Limit,
		"total_pages": query.TotalPages(total),
	})
}

type adminDecideReq struct {
	Action string `json:"action" binding:"required,oneof=approve reject"`
	Reason string `json:"reason" binding:"max=255"`
}

func (h *AdminHandler) AdminDecide(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid approval request id")
		return
	}
	var req adminDecideReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.aprSvc.AdminDecide(c.Request.Context(), mustAdminID(c), id, req.Action == "approve", req.Reason)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// --- reconciliation -----------------------------------------------------------

type reconRunReq struct {
	Date string `json:"date" binding:"required"`
}

func (h *AdminHandler) ReconRun(c *gin.Context) {
	var req reconRunReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	day, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		respondValidation(c, "date must be YYYY-MM-DD")
		return
	}
	runID, err := h.reconSvc.RunForDate(c.Request.Context(), day)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"run_id": runID, "date": req.Date})
}

func (h *AdminHandler) ReconGet(c *gin.Context) {
	var query AdminPaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query.Normalize()
	}
	day, err := time.Parse("2006-01-02", c.Param("date"))
	if err != nil {
		respondValidation(c, "date must be YYYY-MM-DD")
		return
	}
	run, items, err := h.reconSvc.GetRun(c.Request.Context(), day)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"run":         run,
		"items":       items,
		"report_url":  h.reconSvc.SignReportURL(run.ReportUrl),
		"total":       len(items),
		"page":        query.Page,
		"limit":       query.Limit,
		"total_pages": 1,
	})
}

// --- Service Catalog & Offer Factory -----------------------------------------

func (h *AdminHandler) ListServices(c *gin.Context) {
	var query AdminPaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query.Normalize()
	}
	services, total, err := h.productSvc.ListServices(c.Request.Context(), query.ToFilter())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"services":    services,
		"total":       total,
		"page":        query.Page,
		"limit":       query.Limit,
		"total_pages": query.TotalPages(total),
	})
}

func (h *AdminHandler) GetService(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid service id")
		return
	}
	srv, err := h.productSvc.GetService(c.Request.Context(), id)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, srv)
}

type updateServiceStatusReq struct {
	Status string `json:"status" binding:"required,oneof=active maintenance disabled"`
}

func (h *AdminHandler) UpdateServiceStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid service id")
		return
	}
	var req updateServiceStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	srv, err := h.productSvc.UpdateServiceStatus(c.Request.Context(), id, store.ServiceStatus(req.Status))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, srv)
}

func (h *AdminHandler) ListOffers(c *gin.Context) {
	var query AdminPaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query.Normalize()
	}
	offers, total, err := h.productSvc.ListOffers(c.Request.Context(), query.ToFilter())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"offers":      offers,
		"total":       total,
		"page":        query.Page,
		"limit":       query.Limit,
		"total_pages": query.TotalPages(total),
	})
}

func (h *AdminHandler) GetOffer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid offer id")
		return
	}
	off, srvs, err := h.productSvc.GetOfferDetail(c.Request.Context(), id)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"offer": off, "services": srvs})
}

// --- Organizations -----------------------------------------------------------

// ListOrgs is the admin-console org overview — deliberately a separate route
// from GET /v1/org (org.Handler.ListMyOrgs), which is scoped to org
// membership via a user-audience JWT and 404s/401s an admin token, and even
// if it didn't would only ever return orgs the admin happens to personally
// belong to (bug: hpay-admin's OrgsPage was calling /v1/org and getting
// logged out on the audience mismatch).
func (h *AdminHandler) ListOrgs(c *gin.Context) {
	var query AdminPaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query.Normalize()
	}
	rows, total, err := h.productSvc.ListOrganizations(c.Request.Context(), query.ToFilter())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"organizations": rows,
		"total":         total,
		"page":          query.Page,
		"limit":         query.Limit,
		"total_pages":   query.TotalPages(total),
	})
}

type adminUpdatePolicyReq struct {
	Name      string          `json:"name" binding:"required,max=100"`
	MinAmount int64           `json:"min_amount"`
	MaxAmount *int64          `json:"max_amount"`
	Tiers     []approval.Tier `json:"tiers" binding:"required"`
}

// UpdateOrgPolicy is the platform-ops override path (hpay-admin "Tổ chức &
// Policy" edit form). Separate from OrgHandler.CreatePolicy — that one is
// mounted under RequireAuth (user-audience JWT) and calls
// org.Service.CreatePolicy, which requireRole()s the caller as an org_members
// admin; an ops/superadmin token isn't an org member at all, so calling that
// route 401s (and the frontend's global 401 handler was logging admins out
// of this page entirely). This uses org.Service.CreatePolicyAsAdmin instead.
func (h *AdminHandler) UpdateOrgPolicy(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid org id")
		return
	}
	var req adminUpdatePolicyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.orgSvc.CreatePolicyAsAdmin(c.Request.Context(), mustAdminID(c), orgID, req.Name, req.MinAmount, req.MaxAmount, req.Tiers)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

// --- Workflows & Audit -------------------------------------------------------

// workflowStatusAliases maps the short labels WorkflowPage.tsx's status
// select already uses to the real wf_status enum values (docs plan
// workflow-instances-filters) — "waiting" reads better in the UI than the DB's
// "waiting_approval", and clients passing the raw enum value still work.
var workflowStatusAliases = map[string]string{
	"waiting": "waiting_approval",
}

func (h *AdminHandler) ListWorkflows(c *gin.Context) {
	var query AdminPaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query.Normalize()
	}
	status := c.Query("status")
	if status == "all" {
		status = ""
	}
	if mapped, ok := workflowStatusAliases[status]; ok {
		status = mapped
	}
	stuckOnly := c.Query("stuck_only") == "true"
	definition := c.Query("definition")
	if definition == "all" {
		definition = ""
	}
	rows, total, err := h.productSvc.ListWorkflowInstances(c.Request.Context(), query.ToFilter(), product.WorkflowInstanceFilter{
		Status: status, Definition: definition, StuckOnly: stuckOnly,
	})
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"workflows":   rows,
		"total":       total,
		"page":        query.Page,
		"limit":       query.Limit,
		"total_pages": query.TotalPages(total),
	})
}

func (h *AdminHandler) GetWorkflow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid workflow id")
		return
	}
	inst, steps, err := h.productSvc.GetWorkflowInstanceDetail(c.Request.Context(), id)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"instance": inst, "steps": steps})
}

// nonNilAuditLogs coerces a nil slice (sqlc's ListAuditLogs leaves `var items
// []AuditLog` unappended-to for zero matches) to an empty, non-nil one —
// encoding/json marshals nil as `null`, which hpay-admin's useAuditQueries.ts
// misreads as "the call failed" and silently falls back to hardcoded demo
// rows instead of showing a real, correct empty result.
func nonNilAuditLogs(logs []store.AuditLog) []store.AuditLog {
	if logs == nil {
		return []store.AuditLog{}
	}
	return logs
}

func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	var query AdminPaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query.Normalize()
	}
	logs, total, err := h.productSvc.ListAuditLogs(c.Request.Context(), query.ToFilter())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"logs":        nonNilAuditLogs(logs),
		"total":       total,
		"page":        query.Page,
		"limit":       query.Limit,
		"total_pages": query.TotalPages(total),
	})
}
