//go:build windows

package nativeapp

import (
	"context"
	"relevo/internal/wintray"
)

// LaunchGUI opens the native-client controls on Windows and reports whether handled.
func LaunchGUI() bool {
	settings, loadErr := wintray.LoadSettings()
	if loadErr != nil {
		wintray.ShowError(loadErr)
		return true
	}
	frames := make(chan []byte, 1)
	e := wintray.RunLab(wintray.LabOptions{Preview: frames, Embedded: true, InputField: true,
		Title:  "Relevo Display 0.15.4 · Pruebas del cliente propio",
		Intro:  "Requiere nuestra app Relevo verificada y abierta en el iPad. Conexión: " + settings.Link() + ". Usa el campo editable para probar la escritura y las opciones Pantalla/Teclado/Mouse del iPad.",
		Footer: "Multitouch y teclado requieren el cliente propio; Yam conserva gestos agregados. Cámara y micrófono: pruebas locales, no dispositivos virtuales.",
		Check: func() (string, error) {
			if settings.Link() == "WiFi" {
				return "Wi-Fi: elige una IP de Windows en la ventana principal y escanea el QR desde Relevo del iPad. Se valida TLS y token antes de crear la pantalla.", nil
			}
			return checkLabUSB()
		},
		Start: func(s wintray.LabSettings, stop <-chan struct{}, report func(string)) error {
			cfg, e := configForLab(s)
			if e != nil {
				return e
			}
			cfg.Position = settings.Position
			cfg.Transport = settings.Link()
			cfg.LANAddress = settings.LANAddress
			cfg.Pairing = wintray.OpenPairing
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
