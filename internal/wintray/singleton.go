package wintray

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type shellVersion [3]uint32

func parseShellVersion(title string) (shellVersion, error) {
	var v shellVersion
	fields := strings.Fields(title)
	if len(fields) != 3 || fields[0] != "Relevo" || fields[1] != "Display" {
		return v, fmt.Errorf("unrecognized Relevo window title %q", title)
	}
	parts := strings.Split(fields[2], ".")
	if len(parts) != 2 && len(parts) != 3 {
		return v, fmt.Errorf("invalid Relevo version %q", fields[2])
	}
	for i, s := range parts {
		if s == "" {
			return v, fmt.Errorf("empty version component")
		}
		for _, c := range s {
			if c < '0' || c > '9' {
				return v, fmt.Errorf("invalid version component")
			}
		}
		n, e := strconv.ParseUint(s, 10, 32)
		if e != nil {
			return v, e
		}
		v[i] = uint32(n)
	}
	return v, nil
}
func (v shellVersion) older(other shellVersion) bool {
	for i, n := range v {
		if n != other[i] {
			return n < other[i]
		}
	}
	return false
}

type singletonWindow struct {
	main, lab uintptr
	title     string
}
type singletonAPI interface {
	create() (handle uintptr, exists bool, err error)
	close(uintptr)
	window() (singletonWindow, error)
	focus(uintptr)
	command(uintptr, uintptr) error
	wait(context.Context) error
}

// acquireShell closes every duplicate handle before waiting. Keeping even one
// duplicate alive would prevent the old named mutex from disappearing forever.
// Existing sessions finish their ordinary cleanup; no process is terminated.
func acquireShell(ctx context.Context, title string, api singletonAPI) (uintptr, bool, error) {
	current, err := parseShellVersion(title)
	if err != nil {
		return 0, false, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return 0, false, fmt.Errorf("la versión anterior de Relevo todavía está cerrando; no se forzó su salida: %w", err)
		}
		h, exists, err := api.create()
		if err != nil {
			return 0, false, err
		}
		if !exists {
			return h, false, nil
		}
		api.close(h)
		window, err := api.window()
		if err != nil {
			return 0, false, err
		}
		// An older instance can own its mutex before creating its window, or after
		// destroying it while returning from the UI. Wait without closing anything.
		if window.main == 0 {
			if err := api.wait(ctx); err != nil {
				return 0, false, err
			}
			continue
		}
		existing, err := parseShellVersion(window.title)
		if err != nil {
			return 0, false, fmt.Errorf("hay otra instancia de Relevo cuya versión no se puede identificar; ciérrala desde su icono antes de abrir %s: %w", title, err)
		}
		if !existing.older(current) {
			target := window.main
			if window.lab != 0 {
				target = window.lab
			}
			api.focus(target)
			return 0, true, nil
		}
		target, command := window.main, uintptr(6)
		if window.lab != 0 {
			target, command = window.lab, 3
		}
		if err := api.command(target, command); err != nil {
			return 0, false, err
		}
		if err := api.wait(ctx); err != nil {
			return 0, false, fmt.Errorf("la versión anterior de Relevo no terminó de cerrar; conserva su proceso para completar la limpieza: %w", err)
		}
	}
}

var singletonPollInterval = 100 * time.Millisecond
