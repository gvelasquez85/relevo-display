//go:build windows

package wintray

import (
	"fmt"
	"os"
	"path/filepath"
	"relevo/internal/appstate"
	"time"
)

// SaveDiagnostic exports counters/status only. No media or raw input is added.
func SaveDiagnostic(content string) error {
	dir, err := appstate.ConfigDir()
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, "diagnosticos")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, "Relevo-"+time.Now().Format("20060102-150405")+"-*.txt")
	if err != nil {
		return err
	}
	if _, err = file.WriteString(content); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = Open(dir); err != nil {
		return fmt.Errorf("diagnóstico guardado en %s: %w", file.Name(), err)
	}
	return nil
}
