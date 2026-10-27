package wintray

import (
	"image"
	"math"
	"testing"
)

func TestSystemKeyboardBounds(t *testing.T) {
	for _, screen := range []image.Rectangle{image.Rect(1920, 0, 2944, 768), image.Rect(-768, -1024, 0, 0), image.Rect(0, 0, 3840, 2160)} {
		original := image.Rect(0, 0, 1200, 400)
		got := systemKeyboardBounds(screen, original)
		if got.Empty() || !got.In(screen) || got.Max.Y != screen.Max.Y {
			t.Fatalf("bad placement: %v in %v", got, screen)
		}
		if math.Abs(float64(got.Dx())/float64(got.Dy())-3) > .03 {
			t.Fatalf("aspect distorted %v", got)
		}
	}
}
func TestSystemKeyboardBoundsInvalid(t *testing.T) {
	if !systemKeyboardBounds(image.Rectangle{}, image.Rect(0, 0, 3, 1)).Empty() {
		t.Fatal("invalid accepted")
	}
}
