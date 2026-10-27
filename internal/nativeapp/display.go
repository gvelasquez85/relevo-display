package nativeapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"relevo/internal/appstate"
	"relevo/internal/wintray"
	"strings"
	"time"
)

// RunDisplay shares the main GUI's Start/Stop with the compatible receiver.
func RunDisplay(settings wintray.Settings, stop <-chan struct{}, report func(string), version string) (result error) {
	if e := settings.Validate(); e != nil {
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
	started := time.Now()
	var measurement string
	receiveReport := func(text string) {
		if strings.Contains(text, "FPS confirmados") {
			measurement = text
		}
		if report != nil {
			report(text)
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
		diagnostic := struct {
			Version, Started, Client, Transport, Error, LastMeasurement string
			Settings                                                    wintray.Settings
		}{Version: version, Started: started.Format(time.RFC3339), Client: "relevo", Transport: settings.Link(), Settings: settings, LastMeasurement: measurement}
		if result != nil {
			diagnostic.Error = result.Error()
		}
		b, e := json.MarshalIndent(diagnostic, "", "  ")
		if e == nil {
			_ = os.WriteFile(filepath.Join(dir, "last-session.json"), b, 0600)
		}
	}()
	if settings.KeepAwake {
		release, e := wintray.AcquireAwake()
		if e != nil {
			return e
		}
		defer release()
	}
	return runTrial(ctx, trialConfig{Continuous: true, Position: settings.Position, FPS: settings.FPS, Quality: settings.Quality, Mode: settings.Mode, Transport: settings.Link(), LANAddress: settings.LANAddress, Multitouch: settings.Touch, Keyboard: settings.Keyboard, Pointer: settings.Touch, Pairing: wintray.OpenPairing}, receiveReport)
}
