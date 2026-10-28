//go:build windows

package winservice

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const diagnosticKey = `SOFTWARE\RelevoDisplay`
const diagnosticValue = "BootConsoleState"

var diagnosticMu sync.Mutex
var previousDiagnostic string

func consoleStateLabel(state string) string {
	switch state {
	case "waiting_usb":
		return "Acceso del iPad: esperando USB y app compatible"
	case "streaming":
		return "Acceso del iPad: transmisión activa"
	case "desktop_unavailable":
		return "Acceso del iPad: escritorio de acceso no disponible"
	case "waiting_login":
		return "Sesión iniciada: control transferido al agente de usuario"
	case "usb_error":
		return "Acceso del iPad: conexión USB no disponible"
	case "capture_error":
		return "Acceso del iPad: error de captura"
	case "launching":
		return "Acceso del iPad: iniciando proceso de consola"
	case "launch_error":
		return "Acceso del iPad: Windows rechazó el inicio del proceso"
	case "worker_exited":
		return "Acceso del iPad: proceso de consola terminado; reintentando"
	case "session_transition":
		return "Acceso del iPad: esperando consola de acceso"
	case "stopped":
		return "Supervisor de consola detenido"
	}
	return ""
}

// ReportPreloginState stores only fixed, non-sensitive categories for the GUI.
// Passwords, input events, account/device identifiers and raw errors are never
// accepted. Registry ACLs remain Windows' protected machine-software defaults.
func ReportPreloginState(state string) {
	switch state {
	case "waiting_usb", "streaming", "desktop_unavailable", "waiting_login", "usb_error", "capture_error":
		writeConsoleState(state, 0)
		recordPreloginResult(state)
	}
}
func reportConsoleError(state string, err error) {
	var code syscall.Errno
	_ = errors.As(err, &code)
	writeConsoleState(state, uint32(code))
}
func writeConsoleState(state string, code uint32) {
	if consoleStateLabel(state) == "" {
		return
	}
	token, err := systemToken(windows.TOKEN_QUERY)
	if err != nil {
		return
	}
	token.Close()
	value := state + "|" + strconv.FormatUint(uint64(code), 10)
	diagnosticMu.Lock()
	defer diagnosticMu.Unlock()
	if value == previousDiagnostic {
		return
	}
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, diagnosticKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	if key.SetStringValue(diagnosticValue, value) == nil {
		previousDiagnostic = value
	}
}
func readConsoleState() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, diagnosticKey, registry.QUERY_VALUE)
	if err != nil {
		return "Estado del acceso del iPad no disponible"
	}
	defer key.Close()
	text, _, err := key.GetStringValue(diagnosticValue)
	if err != nil || len(text) > 128 {
		return "Estado del acceso del iPad no disponible"
	}
	stage, numeric, ok := strings.Cut(text, "|")
	if !ok {
		return "Estado del acceso del iPad no disponible"
	}
	label := consoleStateLabel(stage)
	if label == "" {
		return "Estado del acceso del iPad no disponible"
	}
	code, err := strconv.ParseUint(numeric, 10, 32)
	if err != nil {
		return "Estado del acceso del iPad no disponible"
	}
	if code != 0 {
		return fmt.Sprintf("%s (Windows %d)", label, code)
	}
	return label
}
func clearConsoleState() {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, diagnosticKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	_ = key.DeleteValue(diagnosticValue)
}

// Keep the pre-login outcome after handoff so the ordinary GUI can diagnose a
// failed boot test without exporting scripts or revealing the sign-in screen.
func resetPreloginResult() {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, diagnosticKey, registry.SET_VALUE)
	if err == nil {
		defer key.Close()
		_ = key.SetStringValue("BootCaptureResult", "not_attempted")
	}
}
func recordPreloginResult(state string) {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, diagnosticKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	previous, _, _ := key.GetStringValue("BootCaptureResult")
	if previous == "streaming" {
		return
	}
	_ = key.SetStringValue("BootCaptureResult", state)
}
func readPreloginResult() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, diagnosticKey, registry.QUERY_VALUE)
	if err != nil {
		return "Captura previa al acceso: sin resultado registrado."
	}
	defer key.Close()
	state, _, _ := key.GetStringValue("BootCaptureResult")
	if state == "streaming" {
		return "Prueba de arranque: se enviaron imágenes antes de iniciar sesión."
	}
	if label := consoleStateLabel(state); label != "" {
		return "Última prueba previa al acceso: " + label
	}
	return "Captura previa al acceso: aún no probada en este arranque."
}
