package testkit

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// TinyPNG is a real 400x400 PNG generated with the standard library. The name is kept
// from the old 1x1 fixture, which the KYC image validator now (rightly) refuses: it
// decodes the file and requires at least 200x200 pixels (docs/specs/w3-kyc-upload).
var TinyPNG = RealTestPNG(400, 400)

func RealTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y += 8 {
		for x := 0; x < w; x += 8 {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}
