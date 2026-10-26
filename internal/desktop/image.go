package desktop

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"sync"
	"time"
)

const maxJPEG = 16 * 1024 * 1024

// EncodeJPEG preserves aspect ratio with black letterboxing at the negotiated size.
func EncodeJPEG(img image.Image) ([]byte, error) {
	return EncodeJPEGSize(img, Width, Height)
}

// EncodeJPEGSize encodes the selected output resolution without distorting its source.
func EncodeJPEGSize(img image.Image, width, height int) ([]byte, error) {
	return EncodeJPEGQuality(img, width, height, 70)
}

// EncodeJPEGQuality avoids resampling an already sized capture frame.
func EncodeJPEGQuality(img image.Image, width, height, quality int) ([]byte, error) {
	if quality < 1 || quality > 100 {
		return nil, errors.New("JPEG quality must be between 1 and 100")
	}
	if img == nil {
		return nil, errors.New("nil source image")
	}
	if width < 1 || height < 1 || width > 4096 || height > 4096 {
		return nil, errors.New("invalid output image dimensions")
	}
	b := img.Bounds()
	if b.Empty() || b.Dx() > 32768 || b.Dy() > 32768 {
		return nil, errors.New("invalid source image dimensions")
	}
	if b.Dx() == width && b.Dy() == height {
		var result bytes.Buffer
		err := jpeg.Encode(&result, img, &jpeg.Options{Quality: quality})
		return result.Bytes(), err
	}
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	w, h := width, height
	if b.Dx()*height > b.Dy()*width {
		h = b.Dy() * width / b.Dx()
	} else {
		w = b.Dx() * height / b.Dy()
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	ox, oy := (width-w)/2, (height-h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.Set(ox+x, oy+y, img.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h))
		}
	}
	var result bytes.Buffer
	if e := jpeg.Encode(&result, out, &jpeg.Options{Quality: quality}); e != nil {
		return nil, e
	}
	return result.Bytes(), nil
}

// ReadJPEG parses one complete JPEG without swallowing bytes from the next frame.
// APP segments are length-delimited: an embedded thumbnail EOI is not a frame end.
func ReadJPEG(r io.Reader) ([]byte, error) {
	var out bytes.Buffer
	buf := make([]byte, 65536)
	read := func(n int) ([]byte, error) {
		if n < 0 || out.Len()+n > maxJPEG {
			return nil, errors.New("JPEG exceeds size limit")
		}
		if n > len(buf) {
			return nil, errors.New("JPEG segment exceeds marker length")
		}
		b := buf[:n]
		if _, e := io.ReadFull(r, b); e != nil {
			return nil, e
		}
		out.Write(b)
		return b, nil
	}
	b, e := read(2)
	if e != nil {
		return nil, e
	}
	if b[0] != 0xff || b[1] != 0xd8 {
		return nil, errors.New("JPEG stream has no SOI marker")
	}
	entropy := false
	for {
		b, e = read(1)
		if e != nil {
			return nil, e
		}
		if b[0] != 0xff {
			if entropy {
				continue
			}
			return nil, errors.New("invalid JPEG marker prefix")
		}
		for {
			b, e = read(1)
			if e != nil {
				return nil, e
			}
			if b[0] != 0xff {
				break
			}
		}
		marker := b[0]
		if entropy && (marker == 0 || marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		if marker == 0xd9 {
			return out.Bytes(), nil
		}
		if marker == 0 || marker == 0xd8 {
			return nil, errors.New("invalid JPEG marker")
		}
		if marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		b, e = read(2)
		if e != nil {
			return nil, e
		}
		n := int(binary.BigEndian.Uint16(b))
		if n < 2 {
			return nil, errors.New("invalid JPEG segment length")
		}
		if _, e = read(n - 2); e != nil {
			return nil, e
		}
		entropy = marker == 0xda
	}
}

type streamSource struct {
	ctx       context.Context
	cancel    context.CancelFunc
	frames    chan []byte
	errors    chan error
	once      sync.Once
	closeFunc func() error
	closeErr  error
}

// NewJPEGStream keeps the latest complete image instead of accumulating stale frames.
// The close callback must close the reader/child to unblock a pending read.
func NewJPEGStream(ctx context.Context, r io.Reader, closeFunc func() error) Source {
	c, cancel := context.WithCancel(ctx)
	s := &streamSource{ctx: c, cancel: cancel, frames: make(chan []byte, 1), errors: make(chan error, 1), closeFunc: closeFunc}
	go func() {
		for {
			b, e := ReadJPEG(r)
			if e != nil {
				select {
				case s.errors <- e:
				default:
				}
				return
			}
			select {
			case <-c.Done():
				return
			default:
			}
			select {
			case s.frames <- b:
			default:
				select {
				case <-s.frames:
				default:
				}
				select {
				case s.frames <- b:
				case <-c.Done():
					return
				}
			}
		}
	}()
	go func() { <-c.Done(); s.Close() }()
	return s
}
func (s *streamSource) Next(ctx context.Context) ([]byte, error) {
	wait, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	select {
	case <-wait.Done():
		return nil, fmt.Errorf("desktop frame: %w", wait.Err())
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	default:
	}
	select {
	case b := <-s.frames:
		return b, nil
	case e := <-s.errors:
		return nil, fmt.Errorf("desktop JPEG stream: %w", e)
	case <-wait.Done():
		return nil, fmt.Errorf("desktop frame: %w", wait.Err())
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}
func (s *streamSource) Close() error {
	s.once.Do(func() {
		s.cancel()
		if s.closeFunc != nil {
			s.closeErr = s.closeFunc()
		}
	})
	return s.closeErr
}
