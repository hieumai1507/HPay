package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

type createKeyReq struct {
	Name     string `json:"name" binding:"required,max=100"`
	Provider string `json:"provider" binding:"required,oneof=anthropic openai"`
	APIKey   string `json:"api_key" binding:"required,max=255"`
}

func (h *AdminHandler) CreateAIKey(c *gin.Context) {
	var req createKeyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	key, err := h.q.InsertAIKey(c.Request.Context(), store.InsertAIKeyParams{
		Name:     req.Name,
		Provider: req.Provider,
		ApiKey:   pgtype.Text{String: req.APIKey, Valid: true},
		IsActive: true,
	})
	if err != nil {
		respondErr(c, err)
		return
	}
	h.auditAdmin(c, "ai.key.create", "ai_key "+key.ID.String(), "provider="+req.Provider+" name="+req.Name)

	c.JSON(http.StatusOK, key)
}

type updateKeyReq struct {
	Name     string `json:"name" binding:"required,max=100"`
	APIKey   string `json:"api_key"`
	IsActive bool   `json:"is_active"`
}

func (h *AdminHandler) UpdateAIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "invalid key id")
		return
	}

	var req updateKeyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	// Fetch old key first to preserve values
	oldKey, err := h.q.GetAIKey(c.Request.Context(), id)
	if err != nil {
		respondErr(c, err)
		return
	}

	apiKey := oldKey.ApiKey
	if req.APIKey != "" {
		apiKey = pgtype.Text{String: req.APIKey, Valid: true}
	}

	key, err := h.q.UpdateAIKey(c.Request.Context(), store.UpdateAIKeyParams{
		ID:       id,
		Name:     req.Name,
		ApiKey:   apiKey,
		IsActive: req.IsActive,
	})
	if err != nil {
		respondErr(c, err)
		return
	}
	// Record WHAT changed, never the key: only whether the secret was replaced.
	h.auditAdmin(c, "ai.key.update", "ai_key "+id.String(),
		"name="+req.Name+" active="+strconv.FormatBool(req.IsActive)+" secret_replaced="+strconv.FormatBool(req.APIKey != ""))

	c.JSON(http.StatusOK, key)
}

func (h *AdminHandler) ListAIKeys(c *gin.Context) {
	keys, err := h.q.ListAIKeys(c.Request.Context())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, keys)
}

func (h *AdminHandler) ListAISettings(c *gin.Context) {
	settings, err := h.q.ListAISettings(c.Request.Context())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

type updateSettingReq struct {
	FunctionName string `json:"function_name" binding:"required,oneof=kyc_scoring fraud_advising"`
	Provider     string `json:"provider" binding:"required,oneof=anthropic openai mock"`
	ModelName    string `json:"model_name" binding:"required,max=100"`
	KeyID        string `json:"key_id"` // UUID string
}

func (h *AdminHandler) UpdateAISetting(c *gin.Context) {
	var req updateSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	keyID := uuid.Nil
	if req.KeyID != "" {
		parsed, err := uuid.Parse(req.KeyID)
		if err != nil {
			respondValidation(c, "invalid key_id UUID")
			return
		}
		// Check if key is active
		key, err := h.q.GetAIKey(c.Request.Context(), parsed)
		if err != nil {
			if err.Error() == "no rows in result set" || err.Error() == "sql: no rows in result relation" {
				c.JSON(http.StatusNotFound, gin.H{"code": "ERR_KEY_NOT_FOUND", "message": "referenced AI key not found"})
				return
			}
			respondErr(c, err)
			return
		}
		if !key.IsActive {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_KEY_INACTIVE", "message": "referenced AI key is inactive"})
			return
		}
		keyID = parsed
	}

	setting, err := h.q.UpdateAISetting(c.Request.Context(), store.UpdateAISettingParams{
		FunctionName: req.FunctionName,
		Provider:     req.Provider,
		ModelName:    req.ModelName,
		KeyID:        keyID,
	})
	if err != nil {
		respondErr(c, err)
		return
	}
	h.auditAdmin(c, "ai.setting.update", "ai_setting "+req.FunctionName,
		"provider="+req.Provider+" model="+req.ModelName+" key_id="+keyID.String())

	c.JSON(http.StatusOK, setting)
}

// auditAdmin writes an audit row for a destructive or configuration action by the
// calling admin (constitution C-08). The reason must never contain secrets; callers
// pass identifiers and non-secret attributes only. A failure to write the row is
// logged and does not undo the action, but is visible to operators.
func (h *AdminHandler) auditAdmin(c *gin.Context, action, resource, reason string) {
	if err := h.q.InsertAudit(c.Request.Context(), store.InsertAuditParams{
		ActorID: mustAdminID(c), ActorType: "admin", Action: action, Resource: resource,
		Reason: pgtype.Text{String: reason, Valid: reason != ""},
		Ip:     pgtype.Text{String: c.ClientIP(), Valid: c.ClientIP() != ""},
	}); err != nil {
		slog.Default().Error("admin audit write failed", "action", action, "err", err)
	}
}
