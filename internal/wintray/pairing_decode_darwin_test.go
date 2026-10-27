//go:build darwin

package wintray

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Uses Apple's actual barcode detector on the GUI bitmap, not QR metadata.
// Only synthetic credentials exist in this test; nothing comes from a session.
func TestPairingGUIQRCodeDecodesWithVision(t *testing.T) {
	if _, e := exec.LookPath("xcrun"); e != nil {
		t.Skip("Apple compiler unavailable; QR pixel decoding not run")
	}
	for _, args := range [][]string{{"--find", "clang"}, {"--sdk", "macosx", "--show-sdk-path"}} {
		if e := exec.Command("xcrun", args...).Run(); e != nil {
			t.Skip("Apple compiler/SDK unavailable; QR pixel decoding not run")
		}
	}
	binary := filepath.Join(t.TempDir(), "pairing-decode")
	build := exec.Command("xcrun", "clang", "-fobjc-arc", "-fmodules", "-framework", "Foundation", "-framework", "Vision", "-framework", "ImageIO", "testdata/pairing_decode.m", "-o", binary)
	if output, e := build.CombinedOutput(); e != nil {
		t.Fatalf("QR detector compile failed: %v\n%s", e, output)
	}
	for _, host := range []string{"10.0.0.1", "192.168.100.250"} {
		t.Run(host, func(t *testing.T) {
			expected := "relevo://pair?host=" + host + "&pin=" + strings.Repeat("ab", 32) + "&port=52743&token=" + strings.Repeat("cd", 16)
			pixels, width, e := pairingPixels(expected)
			if e != nil {
				t.Fatal(e)
			}
			img := image.NewNRGBA(image.Rect(0, 0, width, width))
			// The GUI sends BGRA top-down DIB pixels. Preserve that exact layout in PNG.
			for y := 0; y < width; y++ {
				for x := 0; x < width; x++ {
					i := (y*width + x) * 4
					img.SetNRGBA(x, y, color.NRGBA{R: pixels[i+2], G: pixels[i+1], B: pixels[i], A: pixels[i+3]})
				}
			}
			var encoded bytes.Buffer
			if e = png.Encode(&encoded, img); e != nil {
				t.Fatal(e)
			}
			input, e := json.Marshal(map[string]string{"png": base64.StdEncoding.EncodeToString(encoded.Bytes()), "expected": expected})
			if e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary)
			command.Stdin = bytes.NewReader(input)
			output, e := command.CombinedOutput()
			if e != nil {
				t.Fatalf("rendered GUI QR did not decode: %v", e)
			}
			if string(output) != "DECODED\n" {
				t.Fatal("QR detector returned no exact-match marker")
			}
		})
	}
}
