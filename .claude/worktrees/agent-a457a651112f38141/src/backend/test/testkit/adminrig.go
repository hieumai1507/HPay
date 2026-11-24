package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/hieumai1507/hpay-backend/internal/api"
	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

const RBACSecret = "rbac-test-secret-0123456789-0123456789-abcdef"

type AdminRig struct {
	T      *testing.T
	E      *Env
	Engine *gin.Engine
	Router http.Handler
	Admin  *auth.JWTIssuer
}

func NewAdminRig(t *testing.T) *AdminRig {
	t.Helper()
	pool := testutil.PoolFromEnv(t)
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := config.Config{
		Env: config.EnvCI, Role: config.RoleAll, JWTSecret: RBACSecret,
		MockBankSecret: "x", StorageProvider: "local", KYCStorageDir: t.TempDir(),
		TOTPKey: []byte("0123456789abcdef0123456789abcdef"),
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	r := api.NewRouter(api.Deps{Cfg: cfg, Log: log, Pool: pool, Redis: rdb})
	return &AdminRig{T: t, E: NewEnv(t, ""), Engine: r, Router: r, Admin: auth.NewJWTIssuer(RBACSecret, "admin")}
}

// newAdmin inserts an admin with the given role and returns its id and a token.
func (r *AdminRig) NewAdmin(role, status string) (uuid.UUID, string) {
	r.T.Helper()
	var id uuid.UUID
	email := "rbac-" + uuid.NewString()[:8] + "@HPay.dev"
	if err := r.E.Pool.QueryRow(context.Background(),
		`INSERT INTO admin_users (email, password_hash, full_name, role, status)
		 VALUES ($1, 'x', 'RBAC Test', $2::admin_role, $3::account_status) RETURNING id`, email, role, status).Scan(&id); err != nil {
		r.T.Fatalf("insert Admin: %v", err)
	}
	tok, err := r.Admin.Issue(id, "rbac-test", time.Now())
	if err != nil {
		r.T.Fatal(err)
	}
	return id, tok
}

func (r *AdminRig) Call(method, path, token string, body any) *httptest.ResponseRecorder {
	r.T.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.Router.ServeHTTP(w, req)
	return w
}

func ErrCode(w *httptest.ResponseRecorder) string {
	var b struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return b.Code
}

func (r *AdminRig) SubmitKYC(token string, idCard, selfie []byte) *httptest.ResponseRecorder {
	r.T.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for name, data := range map[string][]byte{"id_card": idCard, "selfie": selfie} {
		part, _ := mw.CreateFormFile(name, name+".bin")
		_, _ = part.Write(data)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/kyc/submit", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.Router.ServeHTTP(w, req)
	return w
}
