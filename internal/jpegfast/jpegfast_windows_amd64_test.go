//go:build windows && amd64

package jpegfast

import (
	"bytes"
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"testing"
)

// This test must run on Windows to validate DLL loading and the x64 ABI, rather
// than merely cross-compiling. Red BGRA also detects incorrect pixel format.
func TestNativeJPEG(t *testing.T) {
	pixels := make([]byte, 32*24*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i+2] = 255
		pixels[i+3] = 255
	}
	encoded, err := EncodeBGRA(pixels, 32, 24, 32*4, 90)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 32 || img.Bounds().Dy() != 24 {
		t.Fatal(img.Bounds())
	}
	r, g, b, _ := img.At(16, 12).RGBA()
	if r < 60000 || g > 4000 || b > 4000 {
		t.Fatalf("incorrect BGRA channel order: %d %d %d", r, g, b)
	}
}

// Run actual lifecycle validation only on Windows. Shutdown is process-final,
// so it belongs after every test that may need the native library.
func TestMain(m *testing.M) {
	code := m.Run()
	dir := native.directory
	if err := Shutdown(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	if dir != "" {
		if _, err := os.Lstat(dir); !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "encoder private directory remains:", err)
			code = 1
		}
	}
	if err := Shutdown(); err != nil {
		fmt.Fprintln(os.Stderr, "repeat shutdown:", err)
		code = 1
	}
	if _, err := EncodeBGRA(make([]byte, 4), 1, 1, 4, 70); !errors.Is(err, ErrUnavailable) {
		fmt.Fprintln(os.Stderr, "encoder callable after shutdown:", err)
		code = 1
	}
	os.Exit(code)
}
