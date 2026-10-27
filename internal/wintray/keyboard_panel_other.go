//go:build !windows

package wintray

import (
	"fmt"
	"relevo/internal/remoteinput"
	"relevo/internal/wininput"
)

func ShowKeyboardPanel(target wininput.Target, send func(remoteinput.Key) error) (func(), error) {
	return nil, fmt.Errorf("el teclado de Relevo requiere Windows")
}
