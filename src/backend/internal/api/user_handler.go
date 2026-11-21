package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/org"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/user"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

// UserHandler serves the m3-api-support read-only endpoints: receiver lookup
// (masked) and own profile with the server-truth transaction limit.
type UserHandler struct {
	q *store.Queries
}

func NewUserHandler(pool *pgxpool.Pool) *UserHandler {
	return &UserHandler{q: store.New(pool)}
}

// Lookup confirms a receiver exists before a transfer, revealing only a masked
// name. 404 is uniform for "no such user" AND "user not active" — existence and
// account state are not distinguishable (anti-enumeration, with rate limit on top).
func (h *UserHandler) Lookup(c *gin.Context) {
	phone := c.Query("phone")
	if phone == "" {
		respondValidation(c, "phone query parameter is required")
		return
	}
	u, err := h.q.GetUserByPhone(c.Request.Context(), phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondErr(c, org.ErrUserNotFound)
			return
		}
		respondErr(c, err)
		return
	}
	if u.Status != store.AccountStatusActive || u.ID.String() == c.GetString("user_id") {
		// Self-lookup also 404s: the transfer flow rejects self-transfer anyway,
		// and answering here would only aid probing.
		respondErr(c, org.ErrUserNotFound)
		return
	}
	c.JSON(http.StatusOK, gin.H{"found": true, "masked_name": user.MaskName(u.FullName)})
}

// Me returns the caller's own profile plus txn_limit derived from
// wallet.TxnLimit — the single source of truth for per-transaction limits
// (clients must not hardcode the tier table).
func (h *UserHandler) Me(c *gin.Context) {
	u, err := h.q.GetUserProfileByID(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         u.ID,
			"phone":      u.Phone,
			"full_name":  u.FullName,
			"kyc_status": u.KycStatus,
		},
		"txn_limit": wallet.TxnLimit(u.KycStatus),
	})
}
