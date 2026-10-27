package wintray

import (
	"strings"
	"testing"
)

func TestPairingQRHasOpaqueWhiteQuietZoneAndBoundedPixels(t *testing.T) {
	p, w, e := pairingPixels("relevo://pair?host=192.168.1.2&port=52743&pin=" + strings.Repeat("0", 64) + "&token=" + strings.Repeat("1", 32))
	if e != nil || w < 200 || w > 320 || len(p) != w*w*4 {
		t.Fatal("invalid QR geometry", w, e)
	}
	black, white := 0, 0
	for y := 0; y < w; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			v := p[i]
			if (v != 0 && v != 255) || p[i+1] != v || p[i+2] != v || p[i+3] != 255 {
				t.Fatal("QR not opaque monochrome")
			}
			if v == 0 {
				black++
			} else {
				white++
			}
			if (x < 4 || x >= w-4 || y < 4 || y >= w-4) && v != 255 {
				t.Fatal("QR quiet zone missing")
			}
		}
	}
	if black == 0 || white == 0 {
		t.Fatal("empty QR")
	}
	if _, _, e := pairingPixels(strings.Repeat("x", 513)); e == nil {
		t.Fatal("oversized QR accepted")
	}
}
