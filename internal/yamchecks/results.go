// Package yamchecks summarizes compatible-client checks without storing input values.
package yamchecks

import (
	"fmt"
	"relevo/internal/desktop"
	"relevo/internal/session"
	"relevo/internal/yaminput"
	"sort"
	"strings"
)

type Result struct {
	Input                               yaminput.Stats
	InputAvailable                      bool
	UserStopped                         bool
	Profile, Resolution, Mode, Position string
	Quality, RequestedFPS               int
	Stats                               session.Stats
	Capture                             desktop.CaptureStatus
	Cleaned                             bool
	Failure                             string
}

func (r Result) Summary() string {
	if r.UserStopped && r.Cleaned {
		return "Prueba detenida por el usuario; recursos liberados. No se marca como completada."
	}
	if !r.Cleaned {
		return "Limpieza de pantalla no confirmada. Revisa el error y Pantallas de Windows."
	}
	if !r.Stats.Ready || r.Stats.FramesSent == 0 {
		return "No se completó una sesión de vídeo; prueba no validada."
	}
	switch r.Profile {
	case "disconnect":
		switch r.Stats.EndReason {
		case session.EndDeviceTerminate, session.EndDeviceLocked, session.EndDeviceBackground, session.EndPeerTimeout, session.EndTransportClosed:
			return "Desconexión detectada y recursos de pantalla liberados. Comprueba visualmente la reorganización del escritorio. Vuelve a abrir Yam y pulsa de nuevo para probar reconexión."
		}
		if r.Failure != "" {
			return "La sesión terminó con un error distinto de desconexión esperada; prueba no validada."
		}
		return "No se observó desconexión. Repite cerrando Yam, bloqueando el iPad o retirando USB durante la prueba."
	case "uac":
		if r.Failure != "" {
			return "La sesión no se mantuvo durante la prueba; recuperación UAC no validada."
		}
		if r.Capture.Pauses > 0 && r.Capture.Recoveries > 0 && !r.Capture.Paused {
			return "Pausa y recuperación observadas sin perder la sesión. Comprueba visualmente la imagen; estos contadores detectan cambio de escritorio, no identifican por sí solos UAC."
		}
		return "Transición de escritorio seguro no observada o incompleta. No se confirma recuperación UAC."
	case "input":
		prefix := ""
		if r.Failure != "" {
			prefix = "Sesión interrumpida; diagnóstico parcial. "
		}
		if !r.InputAvailable {
			return prefix + "Eventos recibidos: " + EventSummary(r.Stats.InputEventCounts) + ". Entrada desactivada o no disponible; no se inyectan acciones ni se guarda texto."
		}
		return prefix + fmt.Sprintf("Entrada: %d eventos procesados, %d omitidos, %d errores. ", r.Input.Handled, r.Input.Ignored, r.Input.Errors) + "Comprueba la respuesta en Windows. Touch de un contacto y gestos interpretados; no confirma multitouch completo. No se guarda texto."
	default:
		if r.Failure != "" {
			return "Prueba interrumpida: " + r.Failure
		}
		return fmt.Sprintf("Medición terminada: %.1f FPS enviados; captura p95 %.1f ms; escritura p95 %.1f ms. Recursos de pantalla liberados. No mide latencia de presentación del iPad.", r.Stats.ActualFPS, r.Stats.CaptureP95Millis, r.Stats.WriteP95Millis)
	}
}
func EventSummary(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for name, n := range counts {
		if n <= 0 {
			continue
		}
		if _, ok := session.InputEventSchema(map[string]interface{}{"eventName": name}); ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "ninguno"
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", name, counts[name]))
	}
	return strings.Join(parts, " · ")
}
func Compare(a, b Result) string {
	if a.Profile != "performance30" || b.Profile != "performance60" || !a.Cleaned || !b.Cleaned || a.Failure != "" || b.Failure != "" || a.UserStopped || b.UserStopped || a.Stats.FramesSent == 0 || b.Stats.FramesSent == 0 || !a.Stats.Ready || !b.Stats.Ready || a.Resolution != b.Resolution || a.Mode != b.Mode || a.Position != b.Position || a.Quality != b.Quality {
		return ""
	}
	return fmt.Sprintf("Comparación %s Q%d: objetivo 30 → %.1f FPS enviados; objetivo 60 → %.1f. Captura p95 %.1f/%.1f ms; escritura p95 %.1f/%.1f ms. El contenido y movimiento deben ser similares; la sensación de lag se comprueba visualmente.", a.Resolution, a.Quality, a.Stats.ActualFPS, b.Stats.ActualFPS, a.Stats.CaptureP95Millis, b.Stats.CaptureP95Millis, a.Stats.WriteP95Millis, b.Stats.WriteP95Millis)
}

func (r Result) ExpectedDisconnect() bool {
	if r.UserStopped || !r.Cleaned || !r.Stats.Ready || r.Stats.FramesSent == 0 {
		return false
	}
	switch r.Stats.EndReason {
	case session.EndDeviceTerminate, session.EndDeviceLocked, session.EndDeviceBackground, session.EndPeerTimeout, session.EndTransportClosed:
		return true
	}
	return false
}
