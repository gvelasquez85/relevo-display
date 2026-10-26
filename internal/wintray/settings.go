package wintray

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"relevo/internal/appstate"
)

type Settings struct {
	Touch      bool   `json:"touch"`
	Keyboard   bool   `json:"keyboard"`
	Mode       string `json:"mode"`
	Position   string `json:"position"`
	KeepAwake  bool   `json:"keep_awake"`
	Resolution string `json:"resolution"`
	FPS        int    `json:"fps"`
	Quality    int    `json:"quality"`
}

var FrameRates = []int{10, 15, 20, 30, 60}
var JPEGQualities = []int{60, 70, 80, 90, 95, 100}

var Resolutions = []string{"1024x768", "1280x960", "1600x1200", "2048x1536"}

func DefaultSettings() Settings {
	return Settings{Touch: true, Keyboard: true, Resolution: "1024x768", FPS: 20, Quality: 70, Mode: "extend", Position: "right"}
}
func (s Settings) Validate() error {
	if s.Mode != "extend" && s.Mode != "desktop" {
		return fmt.Errorf("modo no compatible: %s", s.Mode)
	}
	if s.Position != "right" && s.Position != "left" && s.Position != "above" && s.Position != "below" {
		return fmt.Errorf("posición no compatible: %s", s.Position)
	}
	found := false
	for _, r := range Resolutions {
		if r == s.Resolution {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("resolución no compatible: %s", s.Resolution)
	}
	validFPS := false
	for _, v := range FrameRates {
		if s.FPS == v {
			validFPS = true
		}
	}
	if !validFPS {
		return fmt.Errorf("FPS no compatible: %d", s.FPS)
	}
	validQuality := false
	for _, v := range JPEGQualities {
		if s.Quality == v {
			validQuality = true
		}
	}
	if !validQuality {
		return fmt.Errorf("calidad no compatible: %d", s.Quality)
	}
	return nil
}
func settingsPath() (string, error) {
	d, e := appstate.ConfigDir()
	return filepath.Join(d, "settings.json"), e
}
func LoadSettings() (Settings, error) {
	p, e := settingsPath()
	if e != nil {
		return DefaultSettings(), e
	}
	legacy, e := appstate.LegacySettingsPath()
	if e != nil {
		return DefaultSettings(), e
	}
	return loadSettingsWithLegacy(p, legacy)
}

func loadSettingsWithLegacy(p, legacy string) (Settings, error) {
	// Only absence permits migration. Malformed or inaccessible current settings
	// must remain visible as an error instead of reviving stale preferences.
	if _, e := os.Lstat(p); e == nil || !os.IsNotExist(e) {
		return loadSettings(p)
	}
	if _, e := os.Lstat(legacy); os.IsNotExist(e) {
		return DefaultSettings(), nil
	} else if e != nil {
		return DefaultSettings(), e
	}
	s, e := loadSettings(legacy)
	if e != nil {
		return DefaultSettings(), e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return DefaultSettings(), e
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return DefaultSettings(), e
	}
	// Another process may have saved new settings since our absence check.
	f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(e) {
		return loadSettings(p)
	}
	if e != nil {
		return DefaultSettings(), e
	}
	_, writeErr := f.Write(append(b, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return DefaultSettings(), writeErr
	}
	if closeErr != nil {
		return DefaultSettings(), closeErr
	}
	return s, nil
}
func loadSettings(p string) (Settings, error) {
	s := DefaultSettings()
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return DefaultSettings(), e
	}
	if e = s.Validate(); e != nil {
		return DefaultSettings(), e
	}
	return s, nil
}
func SaveSettings(s Settings) error {
	p, e := settingsPath()
	if e != nil {
		return e
	}
	return saveSettings(p, s)
}
func saveSettings(p string, s Settings) error {
	if e := s.Validate(); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(p, append(b, '\n'), 0600)
}
