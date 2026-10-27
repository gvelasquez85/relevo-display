package wintray

import (
	"fmt"
	"github.com/skip2/go-qrcode"
)

// Pairing pixels remain in memory; no QR or credential is written to disk.
func pairingPixels(uri string) ([]byte, int, error) {
	if len(uri) > 512 {
		return nil, 0, fmt.Errorf("invalid pairing data")
	}
	code, e := qrcode.New(uri, qrcode.Medium)
	if e != nil {
		return nil, 0, fmt.Errorf("could not create pairing QR")
	}
	bitmap := code.Bitmap()
	modules := len(bitmap)
	scale := 320 / modules
	if scale < 1 {
		return nil, 0, fmt.Errorf("pairing QR too large")
	}
	width := modules * scale
	pixels := make([]byte, width*width*4)
	for y := 0; y < width; y++ {
		for x := 0; x < width; x++ {
			v := byte(255)
			if bitmap[y/scale][x/scale] {
				v = 0
			}
			i := (y*width + x) * 4
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = v, v, v, 255
		}
	}
	return pixels, width, nil
}
