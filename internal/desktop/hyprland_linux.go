//go:build linux

package desktop

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Sources: Hyprland 0.54 Using-hyprctl (output create/remove, keyword),
// and ammen99/wf-recorder README (output, codec, muxer, pixel format).
// No persistent compositor configuration is edited.
func hyprMonitorNames(data []byte) (map[string]bool, error) {
	var monitors []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &monitors); err != nil {
		return nil, fmt.Errorf("invalid Hyprland monitor list: %w", err)
	}
	names := make(map[string]bool, len(monitors))
	for _, m := range monitors {
		names[m.Name] = true
	}
	return names, nil
}
func hyprCall(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "hyprctl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("hyprctl failed: %w", err)
	}
	return out, nil
}
func openHyprland(ctx context.Context, opt Options) (Source, error) {
	width, height, err := opt.Dimensions()
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("wf-recorder"); err != nil {
		return nil, fmt.Errorf("Hyprland extended display requires wf-recorder: %w", err)
	}
	before, err := hyprCall(ctx, "-j", "monitors", "all")
	if err != nil {
		return nil, err
	}
	names, err := hyprMonitorNames(before)
	if err != nil {
		return nil, err
	}
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, err
	}
	name := "relevo-" + hex.EncodeToString(random[:])
	if names[name] {
		return nil, fmt.Errorf("refusing to reuse existing Hyprland monitor")
	}
	if _, err = hyprCall(ctx, "output", "create", "headless", name); err != nil {
		return nil, fmt.Errorf("create Hyprland virtual output: %w", err)
	}
	// Ownership is established only by successful creation of an absent,
	// cryptographically unique name. Cleanup never enumerates/removes other outputs.
	var cleanupOnce sync.Once
	var cleanupErr error
	cleanup := func() error {
		cleanupOnce.Do(func() { _, cleanupErr = hyprCall(context.Background(), "output", "remove", name) })
		return cleanupErr
	}
	success := false
	defer func() {
		if !success {
			cleanup()
		}
	}()
	after, err := hyprCall(ctx, "-j", "monitors", "all")
	if err != nil {
		return nil, err
	}
	created, err := hyprMonitorNames(after)
	if err != nil {
		return nil, err
	}
	if !created[name] {
		return nil, fmt.Errorf("Hyprland did not expose the requested virtual output")
	}
	if _, err = hyprCall(ctx, "keyword", "monitor", fmt.Sprintf("%s,%dx%d@60,auto,1", name, width, height)); err != nil {
		return nil, fmt.Errorf("configure virtual output: %w", err)
	}
	// Verify configuration instead of assuming the compositor accepted it.
	current, err := hyprCall(ctx, "-j", "monitors")
	if err != nil {
		return nil, err
	}
	var monitors []struct {
		Name   string `json:"name"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if err = json.Unmarshal(current, &monitors); err != nil {
		return nil, err
	}
	valid := false
	for _, m := range monitors {
		if m.Name == name && m.Width == width && m.Height == height {
			valid = true
		}
	}
	if !valid {
		return nil, fmt.Errorf("virtual output is not active at %dx%d", width, height)
	}
	runctx, stop := context.WithCancel(ctx)
	cmd := exec.CommandContext(runctx, "wf-recorder", hyprRecorderArgs(name, opt.FPS)...)
	cmd.Stderr = io.Discard
	cmd.Stdout = io.Discard
	out, writer, err := os.Pipe()
	if err != nil {
		stop()
		return nil, err
	}
	cmd.ExtraFiles = []*os.File{writer}
	if err = cmd.Start(); err != nil {
		stop()
		out.Close()
		writer.Close()
		return nil, err
	}
	writer.Close()
	source := NewJPEGStream(ctx, bufio.NewReaderSize(out, 64*1024), func() error { stop(); out.Close(); _ = cmd.Wait(); return cleanup() })
	success = true
	if opt.Log != nil {
		opt.Log("Pantalla virtual Hyprland creada: " + name + "; mueve una ventana allí. Se elimina al cerrar la sesión.")
	}
	return source, nil
}
