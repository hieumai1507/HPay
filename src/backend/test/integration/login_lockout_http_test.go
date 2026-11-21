package integration

// BE-H8 over HTTP: the lockout is visible to clients as 429 ERR_LOGIN_LOCKED and the response
// for an unknown phone is the same as for a wrong password.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginHTTP_LockoutAndUniformErrors(t *testing.T) {
	rig := newAdminRig(t)
	reg := func(phone string) {
		w := rig.call("POST", "/v1/auth/register", "", map[string]any{"phone": phone, "password": "correct-horse-1", "full_name": "Lock Http"})
		if w.Code != http.StatusCreated && w.Code != http.StatusOK {
			t.Fatalf("register: %d %s", w.Code, w.Body.String())
		}
	}
	phone := fmt.Sprintf("09%08d", rand.Intn(100_000_000))
	reg(phone)
	login := func(p, pw string) (int, string) {
		b, _ := json.Marshal(map[string]any{"phone": p, "password": pw})
		req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = fmt.Sprintf("10.%d.%d.%d:1234", rand.Intn(250)+1, rand.Intn(250)+1, rand.Intn(250)+1) // a fresh client address: the per-IP limiter is not under test
		w := httptest.NewRecorder()
		rig.router.ServeHTTP(w, req)
		return w.Code, errCode(w)
	}
	// unknown phone and wrong password give the same answer
	c1, e1 := login(fmt.Sprintf("09%08d", rand.Intn(100_000_000)), "wrong-password")
	c2, e2 := login(phone, "wrong-password")
	if c1 != c2 || e1 != e2 {
		t.Fatalf("unknown phone (%d %s) and wrong password (%d %s) must look the same", c1, e1, c2, e2)
	}
	for i := 0; i < 3; i++ {
		login(phone, "wrong-password")
	}
	code, ec := login(phone, "wrong-password") // 5th failure
	if code != http.StatusTooManyRequests || ec != "ERR_LOGIN_LOCKED" {
		t.Fatalf("5th failure: want 429 ERR_LOGIN_LOCKED, got %d %s", code, ec)
	}
	code, ec = login(phone, "correct-horse-1")
	if code != http.StatusTooManyRequests || ec != "ERR_LOGIN_LOCKED" {
		t.Fatalf("correct password while locked: want 429 ERR_LOGIN_LOCKED, got %d %s", code, ec)
	}
}
