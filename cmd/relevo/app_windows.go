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
	"relevo/internal/windriver"
	"relevo/internal/winfocus"
	"relevo/internal/wininput"
	"relevo/internal/wintray"
	"relevo/internal/yaminput"
	"relevo/internal/yamkeyboard"
	"strconv"
	"strings"
	"time"
)

func launchDesktopApp() bool {
	e := wintray.Run(wintray.Options{Title: "Relevo Display 0.15.2", Start: runTraySession, DisconnectMonitor: func() error { return desktop.DisconnectVirtualMonitor(context.Background(), nil) }, OpenYamTests: openYamTests, OpenTests: func() { nativeapp.LaunchGUI() }, ExportDiagnostic: exportLastDiagnostic, OpenKeyboard: openYamKeyboard})
	if e != nil {
		wintray.ShowError(e)
	}
	return true
}

type compatibleRunOptions struct {
	Duration time.Duration
	Observe  func(session.Stats, desktop.CaptureStatus) string
	Complete func(session.Stats, desktop.CaptureStatus, bool, error, yaminput.Stats, bool)
}

func runTraySession(settings wintray.Settings, stop <-chan struct{}, report func(string)) error {
	if settings.Receiver() == "relevo" {
		return nativeapp.RunDisplay(settings, stop, report, version)
	}
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
	var inputStats yaminput.Stats
	var inputAvailable bool
	if opt.Complete != nil {
		defer func() { opt.Complete(stats, captureStatus, cleaned, err, inputStats, inputAvailable) }()
	}
	var selectedEncoder string
	var events []string
	log := func(s string) {
		if len(events) >= 80 {
			events = events[1:]
		}
		events = append(events, time.Now().Format(time.RFC3339)+" "+s)
		if strings.HasPrefix(s, "Windows solicitará permiso") {
			report(s)
		}
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
			RemoteInput                  yaminput.Stats
			RemoteInputAvailable         bool
			MonitorDeviceState           *windriver.DeviceState
			MonitorDeviceStateError      string
		}{Version: version, Resolution: resolution, Started: started.Format(time.RFC3339), Stats: stats, Events: events, RequestedFPS: settings.FPS, JPEGQuality: settings.Quality, Settings: settings, Encoder: selectedEncoder, CaptureCleanupConfirmed: cleaned, CaptureStatus: captureStatus, RemoteInput: inputStats, RemoteInputAvailable: inputAvailable}
		if err != nil {
			result.Error = err.Error()
		}
		if settings.Mode == "extend" {
			state, stateErr := windriver.ReadDeviceState(context.Background())
			if stateErr != nil {
				result.MonitorDeviceStateError = stateErr.Error()
			} else {
				result.MonitorDeviceState = &state
			}
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
	sourceOpened := false
	defer func() {
		if !sourceOpened && settings.Mode == "extend" && ctx.Err() != nil {
			// Stop can arrive during USB discovery, before a capture source
			// exists. It must still remove an already advertised monitor.
			cleanupErr := desktop.DisconnectVirtualMonitor(context.Background(), log)
			cleaned = cleanupErr == nil
			err = errors.Join(normalizeSessionError(ctx, err), cleanupErr)
		}
	}()
	go func(stopCtx context.Context) {
		select {
		case <-stop:
			cancel()
		case <-stopCtx.Done():
		}
	}(ctx)
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
	log("capture activation requested")
	report("Preparando pantalla y captura…")
	source, e := desktop.Open(ctx, desktop.Options{Mode: settings.Mode, Display: 1, Position: settings.Position, FPS: settings.FPS, JPEGQuality: settings.Quality, Width: w, Height: h, Log: log})
	if e != nil {
		err = normalizeSessionError(ctx, e)
		cleaned = err == nil
		log(fmt.Sprintf("capture activation ended: %v", e))
		return err
	}
	sourceOpened = true
	log("capture activation completed")
	defer func() {
		report("Desconectando el dispositivo de pantalla virtual… Windows puede solicitar permiso.")
		log("capture cleanup requested")
		if info, ok := source.(interface{ CaptureStatus() desktop.CaptureStatus }); ok {
			captureStatus = info.CaptureStatus()
		}
		cleanupErr := source.Close()
		cleaned = cleanupErr == nil
		log(fmt.Sprintf("capture cleanup completed: %v", cleanupErr))
		err = errors.Join(err, cleanupErr)
	}()
	// Input owns only contact/key state. Register after source cleanup so it
	// releases held input before the display peripheral is removed (LIFO).
	var input *yaminput.Controller
	var keyboardPanel *yamkeyboard.Manager
	if settings.Touch || settings.Keyboard {
		target, inputErr := desktop.InputTarget(source)
		if inputErr == nil {
			mouse, mouseErr := wininput.NativeBackend()
			extended, extendedErr := wininput.NativeExtendedBackend()
			inputErr = errors.Join(mouseErr, extendedErr)
			if inputErr == nil {
				inputOptions := yaminput.Options{Touch: settings.Touch, Keyboard: settings.Keyboard, NativeGestures: settings.NativeGestures}
				if settings.Keyboard {
					keyboardPanel = yamkeyboard.NewWithContext(target, winfocus.ProbeEditable, func(ctx context.Context, t wininput.Target) (func(), error) {
						return wintray.ShowSystemKeyboardContext(ctx, t)
					}, report, wintray.SystemKeyboardVisible)
					activeYamKeyboard.Store(keyboardPanel)
					defer func() {
						activeYamKeyboard.CompareAndSwap(keyboardPanel, nil)
						keyboardPanel.Close()
					}()
					inputOptions.Tap = keyboardPanel.Tap
				}
				input, inputErr = yaminput.New(target, mouse, extended, inputOptions)
			}
		}
		if inputErr != nil {
			log(fmt.Sprintf("remote input initialization unavailable: %v; video continues (no event values)", inputErr))
		} else {
			inputAvailable = true
			log(fmt.Sprintf("remote input enabled: touch=%t keyboard=%t native_gestures=%t; native gestures reconstruct two contacts from aggregate events", settings.Touch, settings.Keyboard, settings.NativeGestures))
			defer func() {
				if input.Close() != nil {
					log("remote input release failed; display cleanup will continue")
				}
				inputStats = input.Stats()
			}()
		}
	}
	var cancelInput func()
	var receiveInput func(map[string]interface{}) error
	if input != nil {
		cancelInput = func() {
			if keyboardPanel != nil {
				keyboardPanel.Cancel()
			}
			_ = input.CancelActive()
		}
		receiveInput = func(event map[string]interface{}) error {
			if keyboardPanel != nil {
				if name, _ := event["eventName"].(string); name != "SingleTap" && name != "HoverEvent" {
					keyboardPanel.Cancel()
				}
			}
			before := input.Stats().Errors
			_ = input.Handle(event)
			if input.Stats().Errors > before && before == 0 {
				log("remote input paused after injection failure; video continues; retry on next input")
			}
			return nil // UAC/input failure must never terminate the video session.
		}
	}
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
	stats, err = session.Run(ctx, conn, session.Options{InputEvent: receiveInput, InputCancel: cancelInput, Continuous: true, FPS: settings.FPS, Width: w, Height: h, Log: log, Progress: func(st session.Stats) {
		encoder := "JPEG"
		if info, ok := source.(interface{ EncoderName() string }); ok && info.EncoderName() != "" {
			encoder = info.EncoderName()
		}
		selectedEncoder = encoder
		if info, ok := source.(interface{ CaptureStatus() desktop.CaptureStatus }); ok {
			captureStatus = info.CaptureStatus()
		}
		extra := ""
		if input != nil {
			inputStats = input.Stats()
			extra = fmt.Sprintf("\nEntrada: %d procesados · %d omitidos · %d errores", inputStats.Handled, inputStats.Ignored, inputStats.Errors)
			if settings.NativeGestures {
				extra += " · gestos Windows de dos dedos"
			}
		} else if settings.Touch || settings.Keyboard {
			extra = "\nEntrada no disponible; vídeo activo."
		}
		if opt.Observe != nil {
			extra += opt.Observe(st, captureStatus)
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
		b, e = json.Marshal(map[string]string{"Version": version, "Info": "Todavía no se ha completado una sesión de pantalla; el estado actual del dispositivo está en ExportedBy."})
	}
	if e != nil {
		return e
	}
	text, e := diagnosticForExport(b, currentDiagnosticIdentity())
	if e != nil {
		return e
	}
	return wintray.SaveDiagnostic(text)
}
