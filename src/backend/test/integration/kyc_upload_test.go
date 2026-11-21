package integration

// KYC upload through the real router and storage (docs/specs/w3-kyc-upload,
// TC-KYC-UP-001..). The unit tests in internal/kyc and internal/api cover the rules;
// these prove they are wired: validator in Submit, key extension follows content,
// oversize body refused before parsing, and stored files are served with safe headers.

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/auth"
)

func realJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 70}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func (r *adminRig) submitKYC(token string, idCard, selfie []byte) *httptest.ResponseRecorder {
	r.t.Helper()
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
	r.router.ServeHTTP(w, req)
	return w
}

func userToken(t *testing.T, id uuid.UUID) string {
	t.Helper()
	tok, err := auth.NewJWTIssuer(rbacSecret, "user").Issue(id, "kyc-up", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// Real images are accepted; the stored key extension follows the content.
func TestKYCUpload_RealImagesAcceptedWithMatchingKeys(t *testing.T) {
	rig := newAdminRig(t)
	uid, _, _ := rig.e.userWithMoney(t, 0)
	w := rig.submitKYC(userToken(t, uid), realJPEG(t, 640, 480), tinyPNG)
	if w.Code != http.StatusAccepted {
		t.Fatalf("submit with a real jpeg and a real png: %d %s", w.Code, w.Body.String())
	}
	var idKey, selfieKey string
	if err := rig.e.pool.QueryRow(context.Background(), `SELECT id_card_url, selfie_url FROM kyc_requests WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1`, uid).Scan(&idKey, &selfieKey); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(idKey, "/id_card.jpg") {
		t.Errorf("the id card is a JPEG and must be stored as .jpg, got %s", idKey)
	}
	if !strings.HasSuffix(selfieKey, "/selfie.png") {
		t.Errorf("the selfie is a PNG and must be stored as .png, got %s", selfieKey)
	}
}

// A signature followed by garbage is refused end to end, and nothing is stored.
func TestKYCUpload_PolyglotRefusedAndNothingStored(t *testing.T) {
	rig := newAdminRig(t)
	uid, _, _ := rig.e.userWithMoney(t, 0)
	hostile := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("<html><script>alert(document.cookie)</script></html>")...)
	w := rig.submitKYC(userToken(t, uid), hostile, tinyPNG)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a jpeg signature followed by html must be refused with 400, got %d %s", w.Code, w.Body.String())
	}
	var n int
	_ = rig.e.pool.QueryRow(context.Background(), `SELECT count(*) FROM kyc_requests WHERE user_id=$1`, uid).Scan(&n)
	if n != 0 {
		t.Fatalf("a refused upload left %d kyc_requests row(s)", n)
	}
}

// A body above the request cap is refused with 413 before the images are read.
func TestKYCUpload_OversizeBodyRefusedWith413(t *testing.T) {
	rig := newAdminRig(t)
	uid, _, _ := rig.e.userWithMoney(t, 0)
	big := bytes.Repeat([]byte{0x41}, 13<<20)
	w := rig.submitKYC(userToken(t, uid), big, tinyPNG)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("a 13 MiB upload must be refused with 413, got %d", w.Code)
	}
	var b struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	if b.Code != "ERR_PAYLOAD_TOO_LARGE" {
		t.Errorf("error code = %q, want ERR_PAYLOAD_TOO_LARGE", b.Code)
	}
}
