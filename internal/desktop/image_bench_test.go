package desktop

import (
	"image"
	"testing"
)

func BenchmarkEncodeJPEGiPad(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 1024, 768))
	for i := range img.Pix {
		img.Pix[i] = byte(i*17 + i/4096)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := EncodeJPEGQuality(img, 1024, 768, 80); err != nil {
			b.Fatal(err)
		}
	}
}
