package kyc

// KYC image validation (docs/specs/w3-kyc-upload, audit BE-H9). Real images are
// generated with the standard library, so the tests need no binary fixtures.
// Cases: AC-1..AC-5, AC-7.

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func solid(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y += 16 {
		for x := 0; x < w; x += 16 {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	return img
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, solid(w, h), &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, solid(w, h)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// AC-1, AC-7: real images pass and report the extension that matches their content.
func TestInspectImage_AcceptsRealJPEGAndPNG(t *testing.T) {
	ext, err := InspectImage(jpegBytes(t, 640, 480))
	if err != nil || ext != ".jpg" {
		t.Fatalf("jpeg: ext=%q err=%v, want .jpg nil", ext, err)
	}
	ext, err = InspectImage(pngBytes(t, 640, 480))
	if err != nil || ext != ".png" {
		t.Fatalf("png: ext=%q err=%v, want .png nil", ext, err)
	}
}

// AC-2: a valid signature followed by garbage is not an image.
func TestInspectImage_RejectsSignatureWithGarbage(t *testing.T) {
	cases := map[string][]byte{
		"jpeg signature + garbage":  append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte("X"), 4096)...),
		"png signature + garbage":   append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte("X"), 4096)...),
		"jpeg header then html":     append([]byte{0xFF, 0xD8, 0xFF}, []byte("<html><script>alert(1)</script></html>")...),
		"just the 4 byte png magic": {0x89, 'P', 'N', 'G'},
		"just the jpeg magic":       {0xFF, 0xD8, 0xFF},
	}
	for name, data := range cases {
		if _, err := InspectImage(data); !errors.Is(err, ErrBadImage) {
			t.Errorf("%s: want ErrBadImage, got %v", name, err)
		}
	}
}

// AC-3: other formats are refused even though the standard library can decode some of them.
func TestInspectImage_RejectsOtherFormats(t *testing.T) {
	var g bytes.Buffer
	if err := gif.Encode(&g, solid(400, 400), nil); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"gif":       g.Bytes(),
		"svg":       []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="400" height="400"><script>alert(1)</script></svg>`),
		"pdf":       []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n"),
		"html":      []byte("<!doctype html><html><body>hi</body></html>"),
		"zip":       {'P', 'K', 3, 4, 0, 0, 0, 0},
		"exe":       {'M', 'Z', 0x90, 0, 3, 0, 0, 0},
		"empty":     {},
		"text":      []byte("not an image"),
		"riff webp": append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 64)...),
	}
	for name, data := range cases {
		if _, err := InspectImage(data); !errors.Is(err, ErrBadImage) {
			t.Errorf("%s: want ErrBadImage, got %v", name, err)
		}
	}
}

// AC-4: dimension limits are applied before pixels are decoded.
func TestInspectImage_DimensionLimits(t *testing.T) {
	tooSmall := pngBytes(t, 150, 400)
	if _, err := InspectImage(tooSmall); !errors.Is(err, ErrBadImage) {
		t.Errorf("150 px wide must be refused, got %v", err)
	}
	if _, err := InspectImage(pngBytes(t, 400, 199)); !errors.Is(err, ErrBadImage) {
		t.Errorf("199 px tall must be refused")
	}
	// 8001 px wide (a decompression-bomb shape: tiny file, huge declared size); a PNG of a
	// single repeated row compresses to almost nothing.
	if _, err := InspectImage(declaredSizePNG(t, 8001, 300)); !errors.Is(err, ErrBadImage) {
		t.Errorf("8001 px wide must be refused")
	}
	if _, err := InspectImage(declaredSizePNG(t, 7000, 7000)); !errors.Is(err, ErrBadImage) {
		t.Errorf("49 megapixels must be refused even though each side is under 8000")
	}
	if _, err := InspectImage(pngBytes(t, 200, 200)); err != nil {
		t.Errorf("200 x 200 is the minimum and must pass: %v", err)
	}
}

// declaredSizePNG writes a PNG header that declares w x h but whose body is tiny; the
// validator must refuse it from DecodeConfig without decoding pixels.
func declaredSizePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Skipf("cannot build a %dx%d test image on this machine: %v", w, h, err)
	}
	return b.Bytes()
}

// AC-5: a truncated image within the full-decode size is refused.
func TestInspectImage_RejectsTruncatedBody(t *testing.T) {
	full := jpegBytes(t, 800, 600)
	if _, err := InspectImage(full[:len(full)/2]); !errors.Is(err, ErrBadImage) {
		t.Errorf("a truncated jpeg must be refused, got %v", err)
	}
	pfull := pngBytes(t, 800, 600)
	if _, err := InspectImage(pfull[:len(pfull)/2]); !errors.Is(err, ErrBadImage) {
		t.Errorf("a truncated png must be refused, got %v", err)
	}
}

// The byte cap still applies.
func TestInspectImage_ByteCap(t *testing.T) {
	if _, err := InspectImage(append(pngBytes(t, 400, 400), bytes.Repeat([]byte{0}, maxImageBytes)...)); !errors.Is(err, ErrBadImage) {
		t.Error("an image above 5 MiB must be refused")
	}
}

// AC-7: the storage key extension follows the content.
func TestKeysMatchContent(t *testing.T) {
	jk, pk := KeyFor("req", "id_card", jpegBytes(t, 400, 400)), KeyFor("req", "selfie", pngBytes(t, 400, 400))
	if !strings.HasSuffix(jk, "/id_card.jpg") {
		t.Errorf("jpeg key = %s", jk)
	}
	if !strings.HasSuffix(pk, "/selfie.png") {
		t.Errorf("png key = %s", pk)
	}
}

// The format allowlist and the full-decode step overlap for GIF in a normal run: removing only
// the allowlist still leaves the GIF refused later by the jpeg decoder. The test pins the BEHAVIOR
// (a GIF is refused by name) and the mutation check removes both layers to prove it bites.
// The allowlist must not depend on which decoders happen to be linked into the binary.
// This test file imports image/gif (so DecodeConfig recognizes GIF here); production does
// not, and another dependency could register more formats later. InspectImage has to
// refuse every format that is not jpeg or png by NAME.
func TestInspectImage_AllowlistIsByFormatName(t *testing.T) {
	var g bytes.Buffer
	if err := gif.Encode(&g, solid(400, 400), nil); err != nil {
		t.Fatal(err)
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(g.Bytes())); err != nil || format != "gif" {
		t.Fatalf("setup: expected the gif decoder to be registered in this test binary, got format=%q err=%v", format, err)
	}
	if ext, err := InspectImage(g.Bytes()); err == nil {
		t.Fatalf("a GIF decoded successfully by DecodeConfig must still be refused by name, got ext=%q", ext)
	}
}
