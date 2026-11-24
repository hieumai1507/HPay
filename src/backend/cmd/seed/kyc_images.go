package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

// kycDemoImage draws a plain 640x400 PNG with a coloured band and a title line pattern, so the admin KYC
// screen shows an image instead of an empty box. It is a placeholder for demo data only, never a real
// document, and contains no personal data.
func kycDemoImage(band color.RGBA) []byte {
	const w, h = 640, 400
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 243, G: 244, B: 246, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 0, w, 90), &image.Uniform{C: band}, image.Point{}, draw.Src)
	line := color.RGBA{R: 156, G: 163, B: 175, A: 255}
	for i, y := range []int{140, 190, 240, 290, 340} {
		width := 440 - i*45
		draw.Draw(img, image.Rect(60, y, 60+width, y+14), &image.Uniform{C: line}, image.Point{}, draw.Src)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		fatal("kyc demo image: %v", err)
	}
	return buf.Bytes()
}

// writeKYCDemoImages stores the two demo images under the keys the seeded KYC request points to, in the
// same local storage root the server reads from (KYC_STORAGE_DIR, default ./data/kyc). Idempotent.
func writeKYCDemoImages(_ context.Context) {
	root := os.Getenv("KYC_STORAGE_DIR")
	if root == "" {
		root = "./data/kyc"
	}
	for key, band := range map[string]color.RGBA{
		"seed/kyc/id-card.png": {R: 14, G: 144, B: 163, A: 255},
		"seed/kyc/selfie.png":  {R: 124, G: 58, B: 237, A: 255},
	} {
		path := filepath.Join(root, filepath.FromSlash(key))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			fatal("kyc demo image dir: %v", err)
		}
		if err := os.WriteFile(path, kycDemoImage(band), 0o640); err != nil {
			fatal("kyc demo image write: %v", err)
		}
	}
	fmt.Printf("  KYC demo images written under %s\n", root)
}
