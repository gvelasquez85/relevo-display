package nativeapp

import (
	"fmt"
	"relevo/internal/usbmux"
	"relevo/internal/wintray"
	"time"
)

func configForLab(s wintray.LabSettings) (trialConfig, error) {
	if e := s.Validate(); e != nil {
		return trialConfig{}, e
	}
	c := trialConfig{Duration: 5 * time.Minute, FPS: s.FPS, Quality: s.Quality, Mode: "pattern"}
	switch s.Profile {
	case "pattern":
	case "extend", "input":
		if s.Profile == "extend" {
			c.Mode = "extend"
		} else {
			c.Mode = "desktop"
		}
		c.Multitouch = true
		c.Keyboard = true
		c.Pointer = true
	case "audio":
		c.Audio = true
	case "microphone":
		c.Microphone = true
		c.Monitor = true
	case "camera":
		c.Camera = true
	default:
		return trialConfig{}, fmt.Errorf("prueba no reconocida")
	}
	return c, nil
}
func checkLabUSB() (string, error) {
	ds, e := (&usbmux.Client{}).Devices()
	if e != nil {
		return "No se pudo consultar el servicio USB de Apple.", e
	}
	usb, network := 0, 0
	for _, d := range ds {
		if d.Connection == "USB" {
			usb++
		}
		if d.Connection == "Network" {
			network++
		}
	}
	if usb == 0 {
		return fmt.Sprintf("No se detectó un iPad por USB. Dispositivos detectados por red: %d. Conecta y desbloquea el iPad, acepta Confiar y abre Relevo. Wi-Fi se inicia seleccionándolo en la ventana principal con nuestro cliente Relevo.", network), nil
	}
	return fmt.Sprintf("Dispositivos USB detectados: %d. Dispositivos por red: %d. Para probar, abre nuestro cliente Relevo en el iPad e inicia el enlace USB. La detección no confirma que la app esté verificada o abierta.", usb, network), nil
}
