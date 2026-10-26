// Package appstate defines the application's per-user storage location.
package appstate

import (
	"os"
	"path/filepath"
)

func ConfigDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "RelevoDisplay"), nil
}

// LegacySettingsPath is read only: the former product's files are never modified.
func LegacySettingsPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "YamTrial", "settings.json"), nil
}
