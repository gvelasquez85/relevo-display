// Relevo Native is an experimental host for our own iPad client, not Yam.
package nativeapp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"net"
	"os"
	"os/signal"
	"relevo/internal/camerapreview"
	"relevo/internal/desktop"
	"relevo/internal/nativesession"
	"relevo/internal/usbmux"
	"relevo/internal/winaudio"
	"relevo/internal/winfocus"
	"relevo/internal/wininput"
	"relevo/internal/winplayback"
	"strings"
	"time"
)

// RunCLI runs a native-client development session using command-line flags.
func RunCLI() (result error) {
	duration := flag.Duration("duration", 20*time.Second, "test duration, maximum 5m")
	fps := flag.Int("fps", 30, "requested FPS 1..60")
	quality := flag.Int("quality", 70, "JPEG quality 1..100")
	mode := flag.String("mode", "pattern", "pattern, desktop or extend")
	endpoint := flag.String("endpoint", "", "local usbmux endpoint override for tests")
	touch := flag.Bool("touch", false, "enable single-finger Windows mouse input (desktop/extend only)")
	audio := flag.Bool("audio", false, "send Windows default output audio to iPad")
	camera := flag.Bool("camera", false, "receive iPad camera as local preview (not a system webcam)")
	multitouch := flag.Bool("multitouch", false, "enable real Windows multi-contact touch (desktop/extend only)")
	keyboard := flag.Bool("keyboard", false, "enable iPad Unicode keyboard (desktop/extend only)")
	microphone := flag.Bool("microphone", false, "receive iPad microphone diagnostics; no system virtual microphone yet")
	monitor := flag.Bool("microphone-monitor", false, "play iPad microphone on Windows speakers for an explicit test; may cause feedback")
	pointer := flag.Bool("pointer", false, "enable dedicated relative trackpad mode")
	flag.Parse()

	ctx, cancelSignal := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancelSignal()
	return runTrial(ctx, trialConfig{Duration: *duration, FPS: *fps, Quality: *quality, Mode: *mode, Endpoint: *endpoint, Touch: *touch, Audio: *audio, Camera: *camera, Multitouch: *multitouch, Keyboard: *keyboard, Microphone: *microphone, Monitor: *monitor, Pointer: *pointer}, nil)
}

type trialConfig struct {
	Duration                                                                 time.Duration
	FPS, Quality                                                             int
	Mode, Endpoint                                                           string
	Touch, Audio, Camera, Multitouch, Keyboard, Microphone, Monitor, Pointer bool
	PreviewReady                                                             func(string) error
	CameraFrame                                                              func([]byte) error
}

func runTrial(ctx context.Context, cfg trialConfig, report func(string)) (result error) {
	if report == nil {
		report = func(s string) { fmt.Println(s) }
	}
	duration, fps, quality, mode, endpoint := &cfg.Duration, &cfg.FPS, &cfg.Quality, &cfg.Mode, &cfg.Endpoint
	touch, audio, camera, multitouch, keyboard, microphone, monitor, pointer := &cfg.Touch, &cfg.Audio, &cfg.Camera, &cfg.Multitouch, &cfg.Keyboard, &cfg.Microphone, &cfg.Monitor, &cfg.Pointer
	if *monitor {
		*microphone = true
	}
	report("Preparando prueba. Abre nuestro cliente Relevo en el iPad e inicia el enlace USB.")
	if *duration < time.Second || *duration > 5*time.Minute || *fps < 1 || *fps > 60 || *quality < 1 || *quality > 100 {
		return fmt.Errorf("invalid duration, FPS or quality")
	}
	if *mode != "pattern" && *mode != "desktop" && *mode != "extend" {
		return fmt.Errorf("invalid mode")
	}
	if (*touch || *multitouch || *keyboard || *pointer) && *mode == "pattern" {
		return fmt.Errorf("input requires desktop or extend mode")
	}
	if *touch && *multitouch {
		return fmt.Errorf("choose mouse touch or real multitouch, not both")
	}
	if e := localEndpoint(*endpoint); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()
	mux := usbmux.Client{Endpoint: *endpoint}
	ds, e := mux.Devices()
	if e != nil {
		return fmt.Errorf("No se pudo abrir el servicio USB de Apple: %w", e)
	}
	device, e := usbmux.Choose(ds, 0)
	if e != nil {
		return e
	}
	report("iPad detectado por USB. Conectando con el cliente Relevo…")
	conn, e := mux.Connect(device.ID, 52742)
	if e != nil {
		return fmt.Errorf("No se pudo conectar con Relevo del iPad. La app debe estar instalada, verificada y abierta: %w", e)
	}
	report("Enlace USB abierto. Preparando pantalla y funciones…")
	defer conn.Close()
	var input *wininput.Controller
	var extended *wininput.ExtendedController
	var pointerInput *wininput.PointerController
	var inputTarget func() (wininput.Target, error)
	var next func(context.Context) ([]byte, error)
	if *mode == "pattern" {
		img := image.NewRGBA(image.Rect(0, 0, 1024, 768))
		colors := []color.RGBA{{255, 255, 255, 255}, {255, 255, 0, 255}, {0, 255, 255, 255}, {0, 255, 0, 255}, {255, 0, 255, 255}, {255, 0, 0, 255}, {0, 0, 255, 255}, {0, 0, 0, 255}}
		for i, c := range colors {
			draw.Draw(img, image.Rect(i*128, 0, (i+1)*128, 768), &image.Uniform{C: c}, image.Point{}, draw.Src)
		}
		base := img
		frame := 0
		next = func(ctx context.Context) ([]byte, error) {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			moving := image.NewRGBA(base.Bounds())
			draw.Draw(moving, moving.Bounds(), base, image.Point{}, draw.Src)
			x := (frame * 16) % 960
			frame++
			draw.Draw(moving, image.Rect(x, 340, x+64, 428), &image.Uniform{C: color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
			return desktop.EncodeJPEGQuality(moving, 1024, 768, *quality)
		}
	} else {
		source, e := desktop.Open(ctx, desktop.Options{Mode: *mode, Position: "right", Display: 1, FPS: *fps, JPEGQuality: *quality, Width: 1024, Height: 768})
		if e != nil {
			return e
		}
		defer func() {
			if e := source.Close(); e != nil {
				result = errors.Join(result, fmt.Errorf("capture cleanup: %w", e))
			}
		}()
		next = source.Next
		if *pointer {
			target, e := desktop.InputTarget(source)
			if e != nil {
				return e
			}
			inputTarget = target
			b, e := wininput.NativeBackend()
			if e != nil {
				return e
			}
			pointerInput = wininput.NewPointer(target, b)
			defer func() { result = errors.Join(result, pointerInput.Close()) }()
		}
		if *touch {
			target, e := desktop.InputTarget(source)
			if e != nil {
				return e
			}
			backend, e := wininput.NativeBackend()
			if e != nil {
				return e
			}
			input = wininput.New(target, backend)
			defer func() { result = errors.Join(result, input.Close()) }()
		}
		if *multitouch || *keyboard {
			target, e := desktop.InputTarget(source)
			if e != nil {
				return e
			}
			inputTarget = target
			backend, e := wininput.NativeExtendedBackend()
			if e != nil {
				return e
			}
			extended = wininput.NewExtended(target, backend)
			defer func() { result = errors.Join(result, extended.Close()) }()
		}

	}
	opts := nativesession.Options{Progress: func(st nativesession.Stats) {
		report(fmt.Sprintf("Prueba activa · imágenes confirmadas: %d · contactos: %d · teclas: %d · paquetes de micrófono: %d", st.Frames, st.Touches, st.KeyboardEvents, st.MicrophonePackets))
	}, FPS: *fps, Next: next, TouchEnabled: *touch, AudioEnabled: *audio, CameraEnabled: *camera, MultiTouchEnabled: *multitouch, KeyboardEnabled: *keyboard, MicrophoneEnabled: *microphone, PointerEnabled: *pointer, InputModesEnabled: *pointer || *keyboard, InputFocusEnabled: *keyboard}
	if inputTarget != nil && *keyboard {
		opts.EditableAt = func(x, y float64) bool {
			t, e := inputTarget()
			if e != nil {
				return false
			}
			px, py, e := wininput.PhysicalCoordinates(t, x, y)
			if e != nil {
				return false
			}
			editable, e := winfocus.Probe(px, py)
			return e == nil && editable
		}
	}
	if pointerInput != nil {
		opts.OnPointer = pointerInput.Handle
	}
	opts.OnMode = func(mode string) error {
		var e error
		if extended != nil {
			e = errors.Join(e, extended.CancelActive())
		}
		if pointerInput != nil {
			e = errors.Join(e, pointerInput.Reset())
		}
		if input != nil {
			e = errors.Join(e, input.Handle(wininput.Event{Phase: "cancel"}))
		}
		return e
	}
	if input != nil {
		opts.OnTouch = func(t nativesession.TouchEvent) error {
			return input.Handle(wininput.Event{Phase: t.Phase, X: t.X, Y: t.Y})
		}
	}
	if extended != nil {
		opts.OnContacts = extended.HandleContacts
		opts.OnKey = extended.HandleKey
	}
	if *microphone {
		opts.Microphone = func([]byte) error { return nil }
		report("Micrófono habilitado para prueba. Actívalo en el iPad; todavía no es un micrófono del sistema.")
	}
	if *monitor {
		player, e := winplayback.Open()
		if e != nil {
			return e
		}
		defer func() { result = errors.Join(result, player.Close()) }()
		opts.Microphone = player.Packet
		report("Escucha del micrófono preparada. Empieza con volumen bajo y actívalo en el iPad.")
	}
	if *audio {
		capture, e := winaudio.Open(ctx)
		if e != nil {
			return e
		}
		defer func() { result = errors.Join(result, capture.Close()) }()
		opts.Audio = capture.Packets()
	}
	if *camera {
		if cfg.CameraFrame != nil {
			opts.Camera = cfg.CameraFrame
			report("Vista previa preparada en Relevo. Activa cámara y envío USB en el iPad.")
		} else {

			preview, e := camerapreview.New()
			if e != nil {
				return e
			}
			defer func() { result = errors.Join(result, preview.Close()) }()
			opts.Camera = preview.Update
			report("Cámara preparada. Activa la cámara y su envío en el iPad. Vista previa: " + preview.URL())
			if cfg.PreviewReady != nil {
				if e := cfg.PreviewReady(preview.URL()); e != nil {
					return e
				}
			}

		}
	}

	s, e := nativesession.Run(ctx, conn, opts)
	fmt.Printf("Applied-image acknowledgments: %d; touch events: %d; touch enabled: %t\n", s.Frames, s.Touches, *touch)
	fmt.Printf("Keyboard events: %d; microphone packets received: %d\n", s.KeyboardEvents, s.MicrophonePackets)
	if ctx.Err() != nil {
		report("Deteniendo entrada y medios; restaurando la pantalla…")
		return nil
	}
	return e
}
func localEndpoint(ep string) error {
	if ep == "" || strings.HasPrefix(ep, "unix://") {
		return nil
	}
	if !strings.HasPrefix(ep, "tcp://") {
		return fmt.Errorf("endpoint must be local usbmux")
	}
	host, _, e := net.SplitHostPort(strings.TrimPrefix(ep, "tcp://"))
	if e != nil {
		return e
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("endpoint TCP must use a loopback IP")
	}
	return nil
}
