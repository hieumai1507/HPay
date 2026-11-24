package kyc

// KYC image validation (audit BE-H9, docs/specs/w3-kyc-upload/spec.md).
//
// The old check looked at the first 2 to 4 bytes, so a file with a JPEG or PNG
// signature and anything after it was accepted and stored. InspectImage decodes:
// the decoder, not the signature, decides what the file is, and the verdict
// decides the stored file name.

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
)

const (
	minSide   = 200        // pixels, each side
	maxSide   = 8000       // pixels, each side
	maxPixels = 40_000_000 // width x height; refused before any pixel is decoded
	// Images up to this many pixels are decoded fully once, so a truncated or corrupt
	// body is caught. Worst case about 48 MB of memory (12 MP x 4 bytes).
	fullDecodeMaxPixels = 12_000_000
)

// InspectImage proves that data is a real, bounded JPEG or PNG and returns the file
// extension that matches its content (".jpg" or ".png"). Anything else, including a
// valid-looking signature followed by garbage, returns ErrBadImage.
func InspectImage(data []byte) (string, error) {
	if len(data) == 0 || len(data) > maxImageBytes {
		return "", fmt.Errorf("%w: size must be 1B..5MB", ErrBadImage)
	}
	// image.DecodeConfig reads only the header, so dimensions are known before any
	// pixel is decoded (decompression-bomb guard).
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("%w: not a decodable image", ErrBadImage)
	}
	var ext string
	switch format {
	case "jpeg":
		ext = ".jpg"
	case "png":
		ext = ".png"
	default:
		return "", fmt.Errorf("%w: only jpeg/png accepted", ErrBadImage)
	}
	if cfg.Width < minSide || cfg.Height < minSide {
		return "", fmt.Errorf("%w: image must be at least %dx%d pixels", ErrBadImage, minSide, minSide)
	}
	if cfg.Width > maxSide || cfg.Height > maxSide || cfg.Width*cfg.Height > maxPixels {
		return "", fmt.Errorf("%w: image is too large (max %d px per side, %d megapixels)", ErrBadImage, maxSide, maxPixels/1_000_000)
	}
	if cfg.Width*cfg.Height <= fullDecodeMaxPixels {
		var derr error
		if format == "jpeg" {
			_, derr = jpeg.Decode(bytes.NewReader(data))
		} else {
			_, derr = png.Decode(bytes.NewReader(data))
		}
		if derr != nil {
			return "", fmt.Errorf("%w: image is truncated or corrupt", ErrBadImage)
		}
	}
	return ext, nil
}

// KeyFor builds the storage key for one image of a request. The extension comes
// from the content (InspectImage), never from the client's file name.
func KeyFor(requestID, name string, data []byte) string {
	ext, err := InspectImage(data)
	if err != nil {
		ext = ".bin" // never reached by Submit: it validates first
	}
	return requestID + "/" + name + ext
}
