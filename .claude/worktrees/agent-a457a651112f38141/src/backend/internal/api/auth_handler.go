package api

import (
	"context"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/auth"
)

var phoneRe = regexp.MustCompile(`^0\d{9}$`)

type AuthHandler struct {
	svc *auth.Service
	// AfterAuth runs after successful register/login (e.g. EnsureMainWallet).
	AfterAuth func(ctx context.Context, userID uuid.UUID) error
}

func NewAuthHandler(svc *auth.Service) *AuthHandler { return &AuthHandler{svc: svc} }

type registerReq struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	FullName string `json:"full_name" binding:"required,min=2,max=120"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	if !phoneRe.MatchString(req.Phone) {
		respondValidation(c, "phone must match 0xxxxxxxxx")
		return
	}
	user, pair, err := h.svc.Register(c.Request.Context(), req.Phone, req.Password, req.FullName, deviceID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	if h.AfterAuth != nil {
		if err := h.AfterAuth(c.Request.Context(), user.ID); err != nil {
			respondErr(c, err)
			return
		}
	}
	c.JSON(http.StatusCreated, gin.H{"user": user, "tokens": pair})
}

type loginReq struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	user, pair, err := h.svc.Login(c.Request.Context(), req.Phone, req.Password, deviceID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	if h.AfterAuth != nil {
		if err := h.AfterAuth(c.Request.Context(), user.ID); err != nil {
			respondErr(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"user": user, "tokens": pair})
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken, c.ClientIP())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tokens": pair})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func deviceID(c *gin.Context) string {
	if id := c.GetHeader("X-Device-Id"); id != "" {
		return id
	}
	return "unknown"
}
