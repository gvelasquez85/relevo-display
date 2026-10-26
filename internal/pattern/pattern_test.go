package pattern

import (
	"bytes"
	"encoding/hex"
	"image/jpeg"
	"testing"
)

func TestJPEG(t *testing.T) {
	a, err := JPEG(0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got.X != Width || got.Y != Height {
		t.Fatalf("size %v", got)
	}
	b, err := JPEG(1)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("frames identical")
	}
	if _, err := JPEG(-1); err != nil {
		t.Fatal(err)
	}
}
func TestVideoPayload(t *testing.T) {
	got := VideoPayload([]byte{0xff, 0xd8, 0xff, 0xd9})
	want, _ := hex.DecodeString("09000000010000000000000121ffd8ffd9")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x want %x", got, want)
	}
	input := []byte{1, 2, 3}
	output := VideoPayload(input)
	input[0] = 9
	if output[13] != 1 {
		t.Fatal("payload aliases caller memory")
	}
}
