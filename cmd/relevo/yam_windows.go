//go:build windows

package main

import (
	"fmt"
	"relevo/internal/desktop"
	"relevo/internal/session"
	"relevo/internal/usbmux"
	"relevo/internal/wintray"
	"relevo/internal/yamchecks"
)

func openYamTests(base wintray.Settings) {
	var previous30, previous60 *yamchecks.Result
	reportIntro := fmt.Sprintf("Usa Yam abierto por USB. Ajustes de la ventana principal: %s · %s · %s. Para cambiar resolución, posición o modo, vuelve a Relevo.", base.Resolution, base.Mode, base.Position)
	err := wintray.RunLab(wintray.LabOptions{
		Embedded: true, Profiles: wintray.YamProfiles(), Validate: wintray.ValidateYamSettings,
		Title: "Relevo Display 0.12.6 · Pruebas con Yam", Intro: reportIntro,
		Footer:      "Sólo app Yam. Los FPS son enviados, no latencia del iPad. Touch/teclado: diagnóstico sin inyección ni texto guardado.",
		ActionLabel: "Abrir solicitud UAC", ActionProfile: "uac", Action: requestUACProbe,
		Check: func() (string, error) {
			c := &usbmux.Client{}
			devices, e := c.Devices()
			if e != nil {
				return "No se pudo consultar el servicio USB de Apple.", e
			}
			usb, network := 0, 0
			for _, d := range devices {
				if d.Connection == "USB" {
					usb++
				} else {
					network++
				}
			}
			return fmt.Sprintf("Dispositivos USB: %d · otros transportes: %d. Abre Yam en el iPad; detectar USB no confirma que esté listo para vídeo.", usb, network), nil
		},
		Start: func(request wintray.LabSettings, stop <-chan struct{}, report func(string)) error {
			settings, duration, e := yamPlan(base, request)
			if e != nil {
				return e
			}
			report(fmt.Sprintf("Prueba de %.0f s · %s · %s · %s · %d FPS · Q%d", duration.Seconds(), settings.Resolution, settings.Mode, settings.Position, settings.FPS, settings.Quality))
			switch request.Profile {
			case "disconnect":
				report("Durante la prueba cierra Yam, bloquea el iPad o desconecta USB. Se comprobará el cierre de sesión y la liberación de pantalla. Después vuelve a conectar/abrir Yam y pulsa otra prueba para verificar reconexión.")
			case "uac":
				report("Con vídeo activo, pulsa Abrir solicitud UAC. Aceptar o cancelar sólo cierra un proceso de prueba; no instala ni modifica nada. Comprueba si la imagen regresa. Si Windows no usa escritorio seguro, no se dará por observada esa transición.")
			case "input":
				report("Toca, arrastra, desplaza con dos dedos y prueba gestos/teclado si Yam los ofrece. Se cuentan eventos y tipos de campos, nunca texto ni coordenadas. No se controla Windows desde esta prueba.")
			case "display":
				report("Comprueba proporción 4:3 y posición del monitor. El modo desktop muestra la pantalla principal en el iPad; no cambia Windows a clonación nativa.")
			default:
				report("Mueve una ventana o desplaza el mismo contenido en ambas pruebas. Después se comparan 30 y 60 FPS con la misma calidad y resolución.")
			}
			result := yamchecks.Result{Profile: request.Profile, Resolution: settings.Resolution, Mode: settings.Mode, Position: settings.Position, Quality: settings.Quality, RequestedFPS: settings.FPS}
			e = runCompatibleSession(settings, stop, report, compatibleRunOptions{
				Duration: duration,
				Observe: func(st session.Stats, c desktop.CaptureStatus) string {
					return "\n" + fmt.Sprintf("Eventos: %s\nPausas de escritorio: %d · recuperaciones: %d · pausa actual: %t", yamchecks.EventSummary(st.InputEventCounts), c.Pauses, c.Recoveries, c.Paused)
				},
				Complete: func(st session.Stats, c desktop.CaptureStatus, cleaned bool, runErr error) {
					result.Stats = st
					result.Capture = c
					result.Cleaned = cleaned
					if runErr != nil {
						result.Failure = runErr.Error()
					}
				},
			})
			select {
			case <-stop:
				result.UserStopped = true
			default:
			}
			report(result.Summary())
			if request.Profile == "input" {
				report("Esquemas recibidos (sólo tipos): " + yamchecks.SchemaSummary(result.Stats.InputEventSchemas))
			}
			switch request.Profile {
			case "performance30":
				copy := result
				previous30 = &copy
			case "performance60":
				copy := result
				previous60 = &copy
			}
			if previous30 != nil && previous60 != nil {
				if comparison := yamchecks.Compare(*previous30, *previous60); comparison != "" {
					report(comparison)
				}
			}
			if request.Profile == "disconnect" && result.ExpectedDisconnect() {
				return nil
			}
			return e
		},
	})
	if err != nil {
		wintray.ShowError(err)
	}
}
