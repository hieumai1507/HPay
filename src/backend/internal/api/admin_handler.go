package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/fraud"
	"github.com/hieumai1507/hpay-backend/internal/kyc"
	"github.com/hieumai1507/hpay-backend/internal/org"
	"github.com/hieumai1507/hpay-backend/internal/product"
	"github.com/hieumai1507/hpay-backend/internal/recon"
	"github.com/hieumai1507/hpay-backend/internal/reversal"
	"github.com/hieumai1507/hpay-backend/internal/storage"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RequireAdmin validates a Bearer token minted with audience "admin"
// (docs 01 §3.1) and stores the admin id in context.
func RequireAdmin(issuer *auth.JWTIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tok, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "ERR_INVALID_TOKEN", "message": "missing bearer token"})
			return
		}
		adminID, _, err := issuer.Parse(tok) // audience check inside Parse
		if err != nil {
			respondErr(c, err)
			c.Abort()
			return
		}
		c.Set("admin_id", adminID.String())
		c.Next()
	}
}

func mustAdminID(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("admin_id"))
	return id
}

type AdminHandler struct {
	authSvc    *auth.AdminService
	kycSvc     *kyc.AdminService
	fraudSvc   *fraud.AdminService
	reconSvc   *recon.Service
	revSvc     *reversal.Service
	aprSvc     *approval.Service
	productSvc *product.Service
	orgSvc     *org.Service
	files      storage.Store
	signer     *storage.Signer
	q          *store.Queries
}

func NewAdminHandler(authSvc *auth.AdminService, kycSvc *kyc.AdminService, fraudSvc *fraud.AdminService,
	reconSvc *recon.Service, revSvc *reversal.Service, aprSvc *approval.Service, productSvc *product.Service,
	orgSvc *org.Service, files storage.Store, signer *storage.Signer, pool *pgxpool.Pool) *AdminHandler {
	return &AdminHandler{authSvc: authSvc, kycSvc: kycSvc, fraudSvc: fraudSvc, reconSvc: reconSvc,
		revSvc: revSvc, aprSvc: aprSvc, productSvc: productSvc, orgSvc: orgSvc, files: files, signer: signer, q: store.New(pool)}
}

type adminLoginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AdminHandler) Login(c *gin.Context) {
	var req adminLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	info, token, err := h.authSvc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"admin": info, "access_token": token})
}

func (h *AdminHandler) KYCQueue(c *gin.Context) {
	var query AdminPaginationQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query.Normalize()
	}
	tab := query.Tab
	if tab == "" {
		tab = c.Query("tab")
	}
	items, total, err := h.kycSvc.Queue(c.Request.Context(), tab, query.ToFilter())
	if err != nil {
		if strings.Contains(err.Error(), "tab must be") {
			respondValidation(c, err.Error())
			return
		}
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"requests":    items,
		"items":       items,
		"total":       total,
		"page":        query.Page,
		"limit":       query.Limit,
		"total_pages": query.TotalPages(total),
	})
}

type kycReviewReq struct {
	Action string `json:"action" binding:"required,oneof=approve reject"`
	Reason string `json:"reason" binding:"max=255"`
}

func (h *AdminHandler) KYCReview(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid kyc request id")
		return
	}
	var req kycReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	res, err := h.kycSvc.Review(c.Request.Context(), mustAdminID(c), id, req.Action == "approve", req.Reason)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ServeFile streams any stored artifact (KYC image or recon xlsx).
func (h *AdminHandler) ServeFile(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("key"), "/")
	if err := h.signer.Verify(key, c.Query("exp"), c.Query("sig")); err != nil {
		respondErr(c, err)
		return
	}
	data, err := h.files.Get(c.Request.Context(), key)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.Data(http.StatusOK, http.DetectContentType(data), data)
}
