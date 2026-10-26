package desktop

import (
	"bytes"
	"testing"
)

func TestBGRAToRGBAKeepsNativeBacking(t *testing.T) {
	src := []byte{3, 2, 1, 0, 30, 20, 10, 17}
	before := append([]byte(nil), src...)
	dst := make([]byte, len(src))
	bgraToRGBA(dst, src)
	if !bytes.Equal(dst, []byte{1, 2, 3, 255, 10, 20, 30, 255}) {
		t.Fatalf("converted %v", dst)
	}
	if !bytes.Equal(src, before) {
		t.Fatalf("native bitmap mutated: %v", src)
	}
}
