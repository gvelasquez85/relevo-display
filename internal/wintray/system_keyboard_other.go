//go:build !windows

package wintray

import (
	"context"
	"fmt"
	"relevo/internal/wininput"
)

func ShowSystemKeyboard(target wininput.Target) (func(), error) {
	return nil, fmt.Errorf("el teclado del sistema requiere Windows")
}

func SystemKeyboardVisible() bool { return false }
func ShowSystemKeyboardContext(_ context.Context, t wininput.Target) (func(), error) {
	return ShowSystemKeyboard(t)
}
