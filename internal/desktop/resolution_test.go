package desktop

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

func TestDimensions(t *testing.T) {
	for _, tc := range []struct {
		opt     Options
		w, h    int
		invalid bool
	}{
		{Options{Mode: "extend"}, 1024, 768, false},
		{Options{Mode: "desktop"}, 1280, 720, false},
		{Options{Mode: "extend", Width: 2048, Height: 1536}, 2048, 1536, false},
		{Options{Mode: "extend", Width: 1280, Height: 720}, 0, 0, true},
		{Options{Mode: "desktop", Width: 1}, 0, 0, true},
		{Options{Mode: "desktop", Width: 32768, Height: 32768}, 0, 0, true},
	} {
		w, h, err := tc.opt.Dimensions()
		if (err != nil) != tc.invalid || err == nil && (w != tc.w || h != tc.h) {
			t.Fatalf("%+v => %dx%d %v", tc.opt, w, h, err)
		}
	}
}
func TestEncodeJPEGSelectedSize(t *testing.T) {
	b, err := EncodeJPEGSize(image.NewRGBA(image.Rect(0, 0, 1024, 768)), 1024, 768)
	if err != nil {
		t.Fatal(err)
	}
	c, err := jpeg.DecodeConfig(bytes.NewReader(b))
	if err != nil || c.Width != 1024 || c.Height != 768 {
		t.Fatal(c, err)
	}
	if _, err = EncodeJPEGSize(image.NewRGBA(image.Rect(0, 0, 1, 1)), 0, 768); err == nil {
		t.Fatal("invalid output accepted")
	}
}
