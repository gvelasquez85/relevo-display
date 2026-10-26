package appstate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsShareUserConfigRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	t.Setenv("AppData", filepath.Join(root, "appdata"))
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	current, err := ConfigDir()
	if err != nil || current != filepath.Join(base, "RelevoDisplay") {
		t.Fatal(current, err)
	}
	legacy, err := LegacySettingsPath()
	if err != nil || legacy != filepath.Join(base, "YamTrial", "settings.json") {
		t.Fatal(legacy, err)
	}
	if _, err := os.Stat(current); !os.IsNotExist(err) {
		t.Fatal("path lookup created directory", err)
	}
}
