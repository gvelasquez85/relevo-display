// Package desktop supplies explicitly selected desktop image sources.
package desktop

import (
	"context"
	"fmt"
)

const Width = 1280
const Height = 720

type Options struct {
	Mode          string
	Position      string
	Display       int
	FPS           int
	JPEGQuality   int
	Width, Height int
	Log           func(string)
}

type Source interface {
	Next(context.Context) ([]byte, error)
	Close() error
}

// Dimensions resolves the negotiated output size. Extension presets match iPad 2's 4:3 panel.
func (o Options) Dimensions() (int, int, error) {
	w, h := o.Width, o.Height
	if w == 0 && h == 0 {
		if o.Mode == "extend" {
			return 1024, 768, nil
		}
		return Width, Height, nil
	}
	if w < 1 || h < 1 || w > 4096 || h > 4096 {
		return 0, 0, fmt.Errorf("output dimensions must both be between 1 and 4096")
	}
	if o.Mode == "extend" {
		if !(w == 1024 && h == 768 || w == 1280 && h == 960 || w == 1600 && h == 1200 || w == 2048 && h == 1536) {
			return 0, 0, fmt.Errorf("iPad extension supports 1024x768, 1280x960, 1600x1200 or 2048x1536")
		}
	}
	return w, h, nil
}

// Quality resolves zero to the balanced default.
func (o Options) Quality() (int, error) {
	q := o.JPEGQuality
	if q == 0 {
		q = 70
	}
	if q < 1 || q > 100 {
		return 0, fmt.Errorf("JPEG quality must be between 1 and 100")
	}
	return q, nil
}
