package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/hieumai1507/hpay-backend/internal/auth"
)

// RequireAuth validates the Bearer access token and stores identity in context.
func RequireAuth(issuer *auth.JWTIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tok, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "ERR_INVALID_TOKEN", "message": "missing bearer token"})
			return
		}
		uid, device, err := issuer.Parse(tok)
		if err != nil {
			respondErr(c, err)
			c.Abort()
			return
		}
		c.Set("user_id", uid.String())
		c.Set("device_id", device)
		c.Next()
	}
}

// RateLimit is a Redis fixed-window limiter keyed by client IP + route.
// Fail-open on Redis errors: availability over strictness for a limiter
// (money safety never depends on it — docs f2 §5 decisions).
func RateLimit(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rl:%s:%s:%d", c.ClientIP(), c.FullPath(), time.Now().Unix()/int64(window.Seconds()))
		n, err := rdb.Incr(c.Request.Context(), key).Result()
		if err == nil {
			if n == 1 {
				rdb.Expire(c.Request.Context(), key, window)
			}
			if n > int64(limit) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": "ERR_RATE_LIMITED", "message": "too many requests"})
				return
			}
		}
		c.Next()
	}
}

// RateLimitUser is RateLimit keyed per authenticated user instead of per IP —
// used on /v1/users/lookup so phone enumeration can't be spread across users
// behind one NAT nor accelerated by one user with many IPs. Must be mounted
// AFTER RequireAuth (falls back to IP if user_id is somehow absent).
func RateLimitUser(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		who := c.GetString("user_id")
		if who == "" {
			who = c.ClientIP()
		}
		key := fmt.Sprintf("rlu:%s:%s:%d", who, c.FullPath(), time.Now().Unix()/int64(window.Seconds()))
		n, err := rdb.Incr(c.Request.Context(), key).Result()
		if err == nil {
			if n == 1 {
				rdb.Expire(c.Request.Context(), key, window)
			}
			if n > int64(limit) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": "ERR_RATE_LIMITED", "message": "too many requests"})
				return
			}
		}
		c.Next()
	}
}

// RequireIdempotencyKey enforces the Idempotency-Key header on money-creating
// POSTs and exposes it to handlers (E01).
func RequireIdempotencyKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" || len(key) > 64 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"code": "ERR_IDEMPOTENCY_KEY_REQUIRED", "message": "Idempotency-Key header (<=64 chars) is required"})
			return
		}
		c.Set("idempotency_key", key)
		c.Next()
	}
}
