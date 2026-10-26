// Package jpegfast provides an optional native JPEG encoder for captured BGRA frames.
package jpegfast

import (
	"errors"
	"fmt"
)

var ErrUnavailable = errors.New("native JPEG encoder unavailable")

func validate(pixels []byte, width, height, stride, quality int) error {
	if width < 1 || height < 1 || width > 4096 || height > 4096 || stride < width*4 || stride > 4096*4 || quality < 1 || quality > 100 {
		return fmt.Errorf("invalid BGRA dimensions, stride or JPEG quality")
	}
	if len(pixels) < (height-1)*stride+width*4 {
		return fmt.Errorf("BGRA buffer too short")
	}
	return nil
}
