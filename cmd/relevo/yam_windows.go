//go:build windows

package main

import (
	"fmt"
	"relevo/internal/desktop"
	"relevo/internal/session"
	"relevo/internal/usbmux"
	"relevo/internal/wintray"
	"relevo/internal/yamchecks"
	"relevo/internal/yaminput"
)

func openYamTests(base wintray.Settings) {
	base.AutoStart = false // Lab sessions never change login registration.
	base.Client = ""
	base.Transport = ""
	base.LANAddress = "" // Compatible tests always use Yam USB.
	var previous30, previous60 *yamchecks.Result
	reportIntro := fmt.Sprintf("Usa Yam abierto por USB. Ajustes de la ventana principal: %s · %s · %s. Para cambiar resolución, posición o modo, vuelve a Relevo.", base.Resolution, base.Mode, base.Position)
	err := wintray.RunLab(wintray.LabOptions{
		Embedded: true, InputField: true, Profiles: wintray.YamProfiles(), Validate: wintray.ValidateYamSettings,
		Title: "Relevo Display 0.16.0 · Pruebas con Yam", Intro: reportIntro,
		Footer:      "Yam: touch, arrastre, scroll y zoom activos también en la pantalla normal. Teclado Windows contextual en campos nativos y controles Edit accesibles del navegador; botón Abrir teclado para otros campos. No se guarda texto.",
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
				report("Toca, arrastra, desplaza con dos dedos y prueba zoom. Se abre el teclado estándar de Windows en la pantalla enviada al iPad. Se cuentan eventos y tipos de campos, nunca texto ni coordenadas. Las opciones Touch/gestos y Teclado Windows de la ventana principal controlan la entrada en Windows. Arrastra esta ventana al monitor del iPad y toca el campo de prueba para abrir el teclado y escribir; UAC y aplicaciones elevadas pueden rechazar entrada.")
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
				Complete: func(st session.Stats, c desktop.CaptureStatus, cleaned bool, runErr error, input yaminput.Stats, available bool) {
					result.Input = input
					result.InputAvailable = available
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
