// Package pattern creates synthetic test images; it never captures a display.
package pattern

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
)

const Width = 1280
const Height = 720

// JPEG renders color bars and a moving position marker for the requested frame.
func JPEG(frame int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	bars := []color.RGBA{{235, 235, 235, 255}, {235, 220, 20, 255}, {20, 220, 220, 255}, {20, 210, 40, 255}, {220, 30, 210, 255}, {220, 30, 30, 255}, {30, 40, 220, 255}, {20, 20, 20, 255}}
	for i, c := range bars {
		draw.Draw(img, image.Rect(i*Width/len(bars), 0, (i+1)*Width/len(bars), Height), image.NewUniform(c), image.Point{}, draw.Src)
	}
	draw.Draw(img, image.Rect(0, Height-180, Width, Height), image.NewUniform(color.RGBA{18, 24, 35, 255}), image.Point{}, draw.Src)
	drawTitle(img, 40, Height-150, 9)
	// Normalize before multiplying so even negative and extreme frame numbers are safe.
	phase := frame % (Width - 80)
	if phase < 0 {
		phase += Width - 80
	}
	x := (phase * 17) % (Width - 80)
	draw.Draw(img, image.Rect(x, Height-45, x+80, Height-15), image.White, image.Point{}, draw.Src)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// VideoPayload wraps JPEG bytes in Yam Display's JPEG video message body (type 108).
func VideoPayload(jpg []byte) []byte {
	out := make([]byte, 13+len(jpg))
	binary.LittleEndian.PutUint32(out[:4], uint32(len(jpg)+5))
	binary.LittleEndian.PutUint32(out[4:8], 1)
	copy(out[8:13], []byte{0, 0, 0, 1, 0x21})
	copy(out[13:], jpg)
	return out
}

func drawTitle(img *image.RGBA, x, y, scale int) {
	glyphs := map[rune][7]string{
		'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
		'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
		'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
		'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
		'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
		'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
		'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
		'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
		'I': {"11111", "00100", "00100", "00100", "00100", "00100", "11111"},
		'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	}
	for _, r := range "RELEVO" {
		if g, ok := glyphs[r]; ok {
			for row, line := range g {
				for col, pixel := range line {
					if pixel == '1' {
						draw.Draw(img, image.Rect(x+col*scale, y+row*scale, x+(col+1)*scale, y+(row+1)*scale), image.White, image.Point{}, draw.Src)
					}
				}
			}
		}
		x += 6 * scale
	}
}
