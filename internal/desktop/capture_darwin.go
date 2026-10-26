package desktop

import (
	"context"
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type macSource struct {
	display       int
	width, height int
	dir           string
	mu            sync.Mutex
	closed        bool
}

func Open(ctx context.Context, opt Options) (Source, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opt.Mode != "desktop" {
		return nil, fmt.Errorf("macOS: mode %q unsupported; extended desktop requires a virtual-display implementation", opt.Mode)
	}
	if opt.Display < 1 {
		return nil, fmt.Errorf("display must be at least 1")
	}
	width, height, err := opt.Dimensions()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "relevo-desktop-")
	if err != nil {
		return nil, err
	}
	return &macSource{display: opt.Display, dir: dir, width: width, height: height}, nil
}
func (s *macSource) Next(ctx context.Context) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, fmt.Errorf("desktop capture is closed")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	path := filepath.Join(s.dir, "frame.jpg")
	defer os.Remove(path)
	cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-D", strconv.Itoa(s.display), "-t", "jpg", path)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("macOS screen capture failed (check Screen Recording permission and selected display): %w", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("screen capture produced no image; check Screen Recording permission: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > 64<<20 {
		return nil, fmt.Errorf("captured image exceeds 64 MiB")
	}
	config, err := jpeg.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 64*1024*1024 {
		return nil, fmt.Errorf("captured image dimensions exceed limit")
	}
	if _, err = f.Seek(0, 0); err != nil {
		return nil, err
	}
	img, err := jpeg.Decode(f)
	if err != nil {
		return nil, err
	}
	return EncodeJPEGSize(img, s.width, s.height)
}
func (s *macSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return os.RemoveAll(s.dir)
}
