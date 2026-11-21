package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/vault"
)

// VaultHandler exposes the metadata side of savings pockets only. Depositing
// and withdrawing go through the 3-step engine with service_code=VAULT_SAVINGS,
// so vaults inherit OfferGuard, the auth matrix and idempotency instead of
// getting a second, unguarded money path (docs 06).
type VaultHandler struct{ svc *vault.Service }

func NewVaultHandler(svc *vault.Service) *VaultHandler { return &VaultHandler{svc: svc} }

func (h *VaultHandler) List(c *gin.Context) {
	vaults, err := h.svc.List(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"vaults": vaults})
}

type createVaultReq struct {
	Label        string `json:"label" binding:"required,max=60"`
	TargetAmount int64  `json:"target_amount" binding:"omitempty,min=0"`
}

func (h *VaultHandler) Create(c *gin.Context) {
	var req createVaultReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	v, err := h.svc.Create(c.Request.Context(), mustUserID(c), req.Label, req.TargetAmount)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

type setRoundUpTargetReq struct {
	Enabled bool `json:"enabled"`
}

// SetRoundUpTarget designates (or clears) the vault named in the path as the
// caller's round-up destination for the optional ROUND_UP workflow step
// (docs plan round-up-vault).
func (h *VaultHandler) SetRoundUpTarget(c *gin.Context) {
	vaultID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondErr(c, vault.ErrNotFound)
		return
	}
	var req setRoundUpTargetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}
	v, err := h.svc.SetRoundUpTarget(c.Request.Context(), mustUserID(c), vaultID, req.Enabled)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}
