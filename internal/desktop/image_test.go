package desktop

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestEncodeJPEGDimensionsAndLetterbox(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	b, e := EncodeJPEG(img)
	if e != nil {
		t.Fatal(e)
	}
	out, e := jpeg.Decode(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	if out.Bounds().Dx() != Width || out.Bounds().Dy() != Height {
		t.Fatal(out.Bounds())
	}
	r, g, bl, _ := out.At(0, Height/2).RGBA()
	if r > 1000 || g > 1000 || bl > 1000 {
		t.Fatal("missing black letterbox")
	}
	r, g, _, _ = out.At(Width/2, Height/2).RGBA()
	if r < 60000 || g > 2000 {
		t.Fatal("lost source content")
	}
}
func TestReadJPEGDoesNotConsumeNextFrameOrThumbnailEOI(t *testing.T) {
	var b bytes.Buffer
	jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 16, 16)), nil)
	j := b.Bytes()
	// APP1 payload includes EOI-looking bytes; parser must honor segment length.
	segment := []byte{0xff, 0xe1, 0, 6, 1, 0xff, 0xd9, 2}
	first := append(append(append([]byte{}, j[:2]...), segment...), j[2:]...)
	stream := bytes.NewReader(append(append([]byte{}, first...), j...))
	a, e := ReadJPEG(stream)
	if e != nil || !bytes.Equal(a, first) {
		t.Fatalf("first frame %v", e)
	}
	a, e = ReadJPEG(stream)
	if e != nil || !bytes.Equal(a, j) {
		t.Fatalf("second frame %v", e)
	}
	if stream.Len() != 0 {
		t.Fatal("remaining bytes")
	}
}
func TestReadJPEGRejectsBrokenInput(t *testing.T) {
	for _, b := range [][]byte{{1, 2}, {0xff, 0xd8, 0xff, 0xe1, 0, 1}, {0xff, 0xd8, 0xff, 0xe1, 0, 5, 1}, {0xff, 0xd8, 0xff, 0xda, 0, 2, 1, 2}} {
		if _, e := ReadJPEG(bytes.NewReader(b)); e == nil {
			t.Fatalf("accepted %x", b)
		}
	}
}
func TestStreamSourceCancellationAndLatest(t *testing.T) {
	r := newBlockingReader()
	ctx, cancel := context.WithCancel(context.Background())
	s := NewJPEGStream(ctx, r, func() error { return r.Close() })
	cancel()
	if _, e := s.Next(ctx); e == nil {
		t.Fatal("ignored cancel")
	}
	s.Close()
}

type blockingReader struct{ done chan struct{} }

func newBlockingReader() *blockingReader             { return &blockingReader{make(chan struct{})} }
func (r *blockingReader) Read(b []byte) (int, error) { <-r.done; return 0, context.Canceled }
func (r *blockingReader) Close() error {
	select {
	case <-r.done:
	default:
		close(r.done)
	}
	return nil
}

func TestEncodeJPEGExactBoundsMatchesDirectEncoder(t *testing.T) {
	img := image.NewRGBA(image.Rect(11, 23, 43, 47))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 31)
	}
	for _, q := range []int{1, 70, 100} {
		got, err := EncodeJPEGQuality(img, 32, 24, q)
		if err != nil {
			t.Fatal(err)
		}
		var want bytes.Buffer
		if err := jpeg.Encode(&want, img, &jpeg.Options{Quality: q}); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want.Bytes()) {
			t.Fatalf("quality %d: source was unnecessarily transformed", q)
		}
	}
}
func TestJPEGQualityValidation(t *testing.T) {
	if q, e := (Options{}).Quality(); e != nil || q != 70 {
		t.Fatalf("default: %d %v", q, e)
	}
	for _, q := range []int{-1, 101} {
		if _, e := (Options{JPEGQuality: q}).Quality(); e == nil {
			t.Fatalf("accepted %d", q)
		}
		if _, e := EncodeJPEGQuality(image.NewRGBA(image.Rect(0, 0, 2, 2)), 2, 2, q); e == nil {
			t.Fatalf("encoder accepted %d", q)
		}
	}
	if _, e := EncodeJPEGQuality(nil, 2, 2, 70); e == nil {
		t.Fatal("accepted nil image")
	}
}
