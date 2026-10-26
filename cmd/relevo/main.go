package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/jpeg"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"relevo/internal/desktop"
	"relevo/internal/jpegfast"
	"relevo/internal/pattern"
	"relevo/internal/session"
	"relevo/internal/usbmux"
	"relevo/internal/windriver"
	"relevo/internal/wintray"
)

const version = "0.11.0-experimental"

func closeCapture(source desktop.Source, prior error) error {
	if source == nil {
		return prior
	}
	if e := source.Close(); e != nil {
		return errors.Join(prior, fmt.Errorf("restore desktop source: %w", e))
	}
	return prior
}

func main() {
	status := 0
	if !(len(os.Args) == 1 && launchDesktopApp()) {
		status = run()
	}
	if e := jpegfast.Shutdown(); e != nil {
		fmt.Fprintln(os.Stderr, "Native encoder cleanup:", e)
	}
	os.Exit(status)
}

func run() int {
	duration := flag.Duration("duration", 20*time.Second, "video duration (1s..5m)")
	fps := flag.Int("fps", 20, "requested JPEG video frames per second (1..60)")
	doctor := flag.Bool("doctor", false, "list local USB transports without connecting to iPad")
	dev := flag.Uint("device", 0, "usbmux device ID (required if multiple USB devices)")
	endpoint := flag.String("usbmux", "", "override local service endpoint, unix://path or tcp://host:port")
	preview := flag.String("preview", "", "write synthetic JPEG to this file and exit")
	versionFlag := flag.Bool("version", false, "print version and exit")
	logPath := flag.String("log", "", "optional JSON result file (no device serials or raw input)")
	mode := flag.String("mode", "pattern", "source: pattern, desktop (existing display), extend (new virtual display where supported)")
	display := flag.Int("display", 1, "existing monitor number; Linux portal uses the system screen picker")
	checkSource := flag.Bool("check-source", false, "open selected image source, validate one JPEG, clean up and exit without iPad")
	installDriver := flag.Bool("install-driver", false, "Windows administrator: install the bundled verified virtual display driver; no USB session is opened")
	pauseInstall := flag.Bool("pause-install", false, "keep the explicit driver installer console open until Enter")
	guiInstall := flag.Bool("gui-install", false, "show graphical driver installation result")
	width := flag.Int("width", 0, "output width; extend defaults to iPad 2 1024")
	height := flag.Int("height", 0, "output height; extend defaults to iPad 2 768")
	quality := flag.Int("quality", 70, "JPEG quality 1..100 (Windows capture)")
	position := flag.String("position", "right", "virtual display position: right,left,above,below (Windows)")
	flag.Parse()
	if *versionFlag {
		fmt.Println("Relevo Display", version)
		return 0
	}
	fmt.Println("Relevo Display", version, "— experimental compatibility test")
	fmt.Println("Uses the existing Yam Display iPad app. Mode:", *mode)
	if *preview != "" {
		b, e := pattern.JPEG(0)
		if e == nil {
			e = os.WriteFile(*preview, b, 0644)
		}
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		fmt.Println("Pattern saved:", *preview)
		return 0
	}
	if *duration < time.Second || *duration > 5*time.Minute || *fps < 1 || *fps > 60 || *dev > 0xffffffff || *display < 1 || (*mode != "pattern" && *mode != "desktop" && *mode != "extend") {
		fmt.Fprintln(os.Stderr, "Invalid mode, display, duration, FPS or device ID")
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log := func(s string) { fmt.Println(time.Now().Format("15:04:05"), s) }
	if *installDriver {
		var installErr error
		if *guiInstall {
			defer func() { wintray.ShowResult(installErr) }()
		}
		if *pauseInstall {
			defer func() {
				fmt.Println("Press Enter to close the installer.")
				_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
			}()
		}
		if !windriver.Supported() {
			installErr = fmt.Errorf("la instalación del controlador requiere Windows x64 nativo")
			fmt.Fprintln(os.Stderr, installErr)
			return 1
		}
		installCtx, stopInstall := context.WithTimeout(ctx, 3*time.Minute)
		defer stopInstall()
		if e := windriver.Install(installCtx, log); e != nil {
			installErr = e
			fmt.Fprintln(os.Stderr, "Virtual display driver installation failed:", e)
			return 1
		}
		fmt.Println("Driver preparation completed. Open Relevo Display and use the notification icon to start the iPad session.")
		return 0
	}
	captureOptions := desktop.Options{Mode: *mode, Position: *position, Display: *display, FPS: *fps, JPEGQuality: *quality, Width: *width, Height: *height, Log: log}
	outputWidth, outputHeight, sizeErr := captureOptions.Dimensions()
	if sizeErr != nil {
		fmt.Fprintln(os.Stderr, sizeErr)
		return 2
	}
	if *mode == "pattern" && (outputWidth != desktop.Width || outputHeight != desktop.Height) {
		fmt.Fprintln(os.Stderr, "pattern uses 1280x720")
		return 2
	}
	var source desktop.Source
	defer func() {
		if e := closeCapture(source, nil); e != nil {
			fmt.Fprintln(os.Stderr, e)
		}
	}()
	openSource := func() error {
		if *mode == "pattern" {
			return nil
		}
		var err error
		source, err = desktop.Open(ctx, captureOptions)
		return err
	}
	if *checkSource {
		if e := openSource(); e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		var b []byte
		var e error
		if source != nil {
			b, e = source.Next(ctx)
		} else {
			b, e = pattern.JPEG(0)
		}
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		config, e := jpeg.DecodeConfig(bytes.NewReader(b))
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		if config.Width != outputWidth || config.Height != outputHeight {
			fmt.Fprintln(os.Stderr, "source returned unexpected image size")
			return 1
		}
		if e = closeCapture(source, nil); e != nil {
			source = nil
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		source = nil
		fmt.Printf("Source check passed: %s, JPEG %dx%d (%d bytes); image not saved.\n", *mode, config.Width, config.Height, len(b))
		return 0
	}
	c := &usbmux.Client{Endpoint: *endpoint}
	ds, e := c.Devices()
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		fmt.Fprintln(os.Stderr, "Linux: install/start usbmuxd. Windows: install Apple mobile device support (Apple Devices/iTunes). Unlock iPad and trust this computer.")
		return 1
	}
	for _, d := range ds {
		fmt.Printf("Transport: ID %d, %s\n", d.ID, d.Connection)
	}
	if *doctor {
		fmt.Println("Diagnostic complete; no iPad connection opened.")
		return 0
	}
	d, e := usbmux.Choose(ds, uint32(*dev))
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	if e = openSource(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	if source != nil {
		fmt.Println("Sharing selected desktop source with iPad for the requested duration.")
	}
	fmt.Printf("Connecting to receiver port 23471 on device ID %d. Keep Yam Display open on iPad; close the original desktop server.\n", d.ID)
	conn, e := c.Connect(d.ID, 23471)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	fmt.Println("USB tunnel connected.")
	options := session.Options{Duration: *duration, FPS: *fps, Log: log, Width: outputWidth, Height: outputHeight}
	if source != nil {
		options.Frame = func(ctx context.Context, index int) ([]byte, error) { return source.Next(ctx) }
	}
	st, e := session.Run(ctx, conn, options)
	e = closeCapture(source, e)
	source = nil
	result := struct {
		Version         string        `json:"version"`
		Stats           session.Stats `json:"stats"`
		Error           string        `json:"error,omitempty"`
		DisplayVerified bool          `json:"display_verified"`
		Mode            string        `json:"mode"`
	}{Version: version, Stats: st, Mode: *mode}
	if e != nil {
		result.Error = e.Error()
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
	if *logPath != "" {
		if w := os.WriteFile(filepath.Clean(*logPath), append(b, '\n'), 0644); w != nil {
			fmt.Fprintln(os.Stderr, "Cannot save result:", w)
			return 1
		}
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "Trial failed:", e)
		return 1
	}
	fmt.Println("Video transmitted. Rendering on iPad requires visual confirmation; successful writes alone do not prove decoding.")
	return 0
}
