//go:build windows

package main

import (
	"fmt"
	"relevo/internal/yamkeyboard"
	"sync/atomic"
)

var activeYamKeyboard atomic.Pointer[yamkeyboard.Manager]

func openYamKeyboard() error {
	m := activeYamKeyboard.Load()
	if m == nil {
		return fmt.Errorf("inicia la pantalla con Yam por USB y habilita Teclado Relevo; después vuelve a pulsar Abrir teclado")
	}
	m.Manual()
	return nil
}
