//go:build windows

package nativeapp

import (
	"context"
	"relevo/internal/wintray"
)

// LaunchGUI opens the native-client controls on Windows and reports whether handled.
func LaunchGUI() bool {
	frames := make(chan []byte, 1)
	e := wintray.RunLab(wintray.LabOptions{Preview: frames, Embedded: true,
		Check: checkLabUSB,
		Start: func(s wintray.LabSettings, stop <-chan struct{}, report func(string)) error {
			cfg, e := configForLab(s)
			if e != nil {
				return e
			}
			if cfg.Camera {
				cfg.CameraFrame = func(b []byte) error {
					frame := append([]byte(nil), b...)
					select {
					case frames <- frame:
					default:
						select {
						case <-frames:
						default:
						}
						select {
						case frames <- frame:
						default:
						}
					}
					return nil
				}
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
			return runTrial(ctx, cfg, report)
		},
	})
	if e != nil {
		wintray.ShowError(e)
	}
	return true
}
