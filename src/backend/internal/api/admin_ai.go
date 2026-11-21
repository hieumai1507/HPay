package api

import (
	"net/http"

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

	c.JSON(http.StatusOK, setting)
}
