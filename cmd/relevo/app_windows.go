//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"relevo/internal/appstate"
	"relevo/internal/desktop"
	"relevo/internal/nativeapp"
	"relevo/internal/session"
	"relevo/internal/usbmux"
	"relevo/internal/wintray"
	"strconv"
	"strings"
	"time"
)

func launchDesktopApp() bool {
	e := wintray.Run(wintray.Options{Title: "Relevo Display 0.12", Start: runTraySession, OpenYamTests: openYamTests, OpenTests: func() { nativeapp.LaunchGUI() }, ExportDiagnostic: exportLastDiagnostic})
	if e != nil {
		wintray.ShowError(e)
	}
	return true
}

type compatibleRunOptions struct {
	Duration time.Duration
	Observe  func(session.Stats, desktop.CaptureStatus) string
	Complete func(session.Stats, desktop.CaptureStatus, bool, error)
}

func runTraySession(settings wintray.Settings, stop <-chan struct{}, report func(string)) error {
	return runCompatibleSession(settings, stop, report, compatibleRunOptions{})
}
func runCompatibleSession(settings wintray.Settings, stop <-chan struct{}, report func(string), opt compatibleRunOptions) (err error) {
	if e := settings.Validate(); e != nil {
		return e
	}
	resolution := settings.Resolution
	if report == nil {
		report = func(string) {}
	}
	report("Comprobando conexión USB…")
	started := time.Now()
	var stats session.Stats
	var captureStatus desktop.CaptureStatus
	var cleaned bool
	if opt.Complete != nil {
		defer func() { opt.Complete(stats, captureStatus, cleaned, err) }()
	}
	var selectedEncoder string
	var events []string
	log := func(s string) {
		if len(events) >= 80 {
			events = events[1:]
		}
		events = append(events, time.Now().Format(time.RFC3339)+" "+s)
	}
	defer func() {
		dir, e := appstate.ConfigDir()
		if e != nil {
			return
		}
		if os.MkdirAll(dir, 0700) != nil {
			return
		}
		result := struct {
			Version, Resolution, Started string
			RequestedFPS, JPEGQuality    int
			Stats                        session.Stats
			Events                       []string
			Error                        string
			CaptureCleanupConfirmed      bool
			CaptureStatus                desktop.CaptureStatus
			Settings                     wintray.Settings
			Encoder                      string
		}{Version: version, Resolution: resolution, Started: started.Format(time.RFC3339), Stats: stats, Events: events, RequestedFPS: settings.FPS, JPEGQuality: settings.Quality, Settings: settings, Encoder: selectedEncoder, CaptureCleanupConfirmed: cleaned, CaptureStatus: captureStatus}
		if err != nil {
			result.Error = err.Error()
		}
		b, e := json.MarshalIndent(result, "", "  ")
		if e == nil {
			_ = os.WriteFile(filepath.Join(dir, "last-session.json"), b, 0600)
		}
	}()

	parts := strings.Split(resolution, "x")
	if len(parts) != 2 {
		return fmt.Errorf("resolución inválida")
	}
	w, e := strconv.Atoi(parts[0])
	if e != nil {
		return e
	}
	h, e := strconv.Atoi(parts[1])
	if e != nil {
		return e
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	// Check the local Apple transport before changing the desktop topology.
	client := &usbmux.Client{}
	devices, e := client.Devices()
	if e != nil {
		return fmt.Errorf("No se pudo abrir el servicio USB de Apple. Pulsa Ayuda USB en la ventana para consultar la guía oficial de Apple. Conecta y desbloquea el iPad y acepta Confiar. Detalle: %w", e)
	}
	device, e := usbmux.Choose(devices, 0)
	if e != nil {
		return fmt.Errorf("Conecta un solo iPad por USB, desbloquéalo y acepta Confiar. %w", e)
	}
	conn, e := client.Connect(device.ID, 23471)
	if e != nil {
		return fmt.Errorf("Abre la app compatible en el iPad y vuelve a iniciar la extensión. %w", e)
	}
	defer conn.Close()
	source, e := desktop.Open(ctx, desktop.Options{Mode: settings.Mode, Display: 1, Position: settings.Position, FPS: settings.FPS, JPEGQuality: settings.Quality, Width: w, Height: h, Log: log})
	if e != nil {
		return e
	}
	defer func() {
		if info, ok := source.(interface{ CaptureStatus() desktop.CaptureStatus }); ok {
			captureStatus = info.CaptureStatus()
		}
		cleanupErr := source.Close()
		cleaned = cleanupErr == nil
		err = errors.Join(err, cleanupErr)
	}()
	if opt.Duration > 0 {
		timedCtx, stopTimer := context.WithTimeout(ctx, opt.Duration)
		defer stopTimer()
		ctx = timedCtx
	}
	if settings.KeepAwake {
		release, e := wintray.AcquireAwake()
		if e != nil {
			return e
		}
		defer release()
	}
	stats, err = session.Run(ctx, conn, session.Options{Continuous: true, FPS: settings.FPS, Width: w, Height: h, Log: log, Progress: func(st session.Stats) {
		encoder := "JPEG"
		if info, ok := source.(interface{ EncoderName() string }); ok && info.EncoderName() != "" {
			encoder = info.EncoderName()
		}
		selectedEncoder = encoder
		if info, ok := source.(interface{ CaptureStatus() desktop.CaptureStatus }); ok {
			captureStatus = info.CaptureStatus()
		}
		extra := ""
		if opt.Observe != nil {
			extra = opt.Observe(st, captureStatus)
		}
		report(fmt.Sprintf("%.1f / %d FPS · %s · %s · Q%d\nCaptura p95: %.1f ms · escritura p95: %.1f ms", st.ActualFPS, settings.FPS, resolution, encoder, settings.Quality, st.CaptureP95Millis, st.WriteP95Millis) + extra)
	}, Frame: func(ctx context.Context, _ int) ([]byte, error) { return source.Next(ctx) }})
	return normalizeSessionError(ctx, err)
}

func exportLastDiagnostic() error {
	dir, e := appstate.ConfigDir()
	if e != nil {
		return e
	}
	b, e := os.ReadFile(filepath.Join(dir, "last-session.json"))
	if os.IsNotExist(e) {
		return wintray.SaveDiagnostic("Relevo Display 0.12: todavía no se ha completado una sesión de pantalla.")
	}
	if e != nil {
		return e
	}
	return wintray.SaveDiagnostic(string(b))
}
