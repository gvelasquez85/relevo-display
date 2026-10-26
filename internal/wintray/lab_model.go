package wintray

import (
	"bytes"
	"fmt"
	"image/jpeg"
)

// LabSettings selects one bounded native-client test.
type LabSettings struct {
	Profile string
	FPS     int
	Quality int
}

// LabOptions supplies the native session and USB diagnostic without coupling the UI to transport.
type LabOptions struct {
	// Embedded returns to the main Relevo window on close.
	Embedded bool
	// Preview carries bounded latest JPEG frames; the producer retains ownership until send.
	Preview <-chan []byte
	Start   func(LabSettings, <-chan struct{}, func(string)) error
	Check   func() (string, error)
}

var labProfiles = []struct{ ID, Label string }{
	{"pattern", "Pantalla de prueba"},
	{"extend", "Pantalla extendida + entrada"},
	{"input", "Touch/teclado/mouse (pantalla actual)"},
	{"audio", "Escuchar Windows en iPad"},
	{"microphone", "Escuchar micrófono del iPad"},
	{"camera", "Cámara del iPad (vista previa)"},
}
var labFPS = []int{30, 60}
var labQuality = []int{60, 70, 80, 90, 95, 100}

func (s LabSettings) Validate() error {
	known := false
	for _, p := range labProfiles {
		known = known || s.Profile == p.ID
	}
	if !known {
		return fmt.Errorf("prueba desconocida: %s", s.Profile)
	}
	if s.FPS != 30 && s.FPS != 60 {
		return fmt.Errorf("selecciona 30 o 60 FPS")
	}
	for _, q := range labQuality {
		if s.Quality == q {
			return nil
		}
	}
	return fmt.Errorf("calidad JPEG no admitida: %d", s.Quality)
}

// decodeLabPreview validates dimensions before allocation and returns top-down BGRA.
func decodeLabPreview(data []byte) ([]byte, int, int, error) {
	if len(data) == 0 || len(data) > 512*1024 {
		return nil, 0, 0, fmt.Errorf("invalid camera frame size")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 640 || cfg.Height > 480 {
		return nil, 0, 0, fmt.Errorf("camera frame exceeds 640x480")
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	pixels := make([]byte, cfg.Width*cfg.Height*4)
	for y := 0; y < cfg.Height; y++ {
		for x := 0; x < cfg.Width; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			i := (y*cfg.Width + x) * 4
			pixels[i] = byte(b >> 8)
			pixels[i+1] = byte(g >> 8)
			pixels[i+2] = byte(r >> 8)
			pixels[i+3] = 255
		}
	}
	return pixels, cfg.Width, cfg.Height, nil
}
