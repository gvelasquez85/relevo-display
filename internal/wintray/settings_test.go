package wintray

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "settings.json")
	want := Settings{Resolution: "1280x960", FPS: 30, Quality: 60, Mode: "extend", Position: "right"}
	if e := saveSettings(p, want); e != nil {
		t.Fatal(e)
	}
	got, e := loadSettings(p)
	if e != nil || got != want {
		t.Fatalf("%+v %v", got, e)
	}
}
func TestSettingsMissingAndInvalidFallback(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	s, e := loadSettings(p)
	if e != nil || s != DefaultSettings() {
		t.Fatal(s, e)
	}
	for _, v := range []string{`{`, `{"resolution":"1x1","fps":20,"quality":70}`, `{"fps":120}`, `{"quality":101}`} {
		if e := os.WriteFile(p, []byte(v), 0600); e != nil {
			t.Fatal(e)
		}
		s, e = loadSettings(p)
		if e == nil || s != DefaultSettings() {
			t.Fatalf("input %s: %+v %v", v, s, e)
		}
	}
}
func TestSettingsInvalidCannotOverwrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	s := DefaultSettings()
	if e := saveSettings(p, s); e != nil {
		t.Fatal(e)
	}
	bad := s
	bad.FPS = 999
	if e := saveSettings(p, bad); e == nil {
		t.Fatal("accepted invalid fps")
	}
	got, e := loadSettings(p)
	if e != nil || got != s {
		t.Fatal(got, e)
	}
}

func TestLegacyMigration(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "RelevoDisplay", "settings.json")
	legacy := filepath.Join(root, "YamTrial", "settings.json")
	want := Settings{Resolution: "1600x1200", FPS: 30, Quality: 80, Mode: "extend", Position: "right"}
	if err := saveSettings(legacy, want); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(legacy)
	got, err := loadSettingsWithLegacy(current, legacy)
	if err != nil || got != want {
		t.Fatal(got, err)
	}
	persisted, err := loadSettings(current)
	if err != nil || persisted != want {
		t.Fatal(persisted, err)
	}
	after, _ := os.ReadFile(legacy)
	if string(before) != string(after) {
		t.Fatal("legacy settings modified")
	}
}

func TestLegacyNeverOverridesCurrentSettings(t *testing.T) {
	for _, contents := range []string{`{"resolution":"1024x768","fps":15,"quality":60}`, `{`, `{"fps":999}`} {
		t.Run(contents, func(t *testing.T) {
			root := t.TempDir()
			current := filepath.Join(root, "settings.json")
			legacy := filepath.Join(root, "old.json")
			if err := saveSettings(legacy, Settings{Resolution: "1600x1200", FPS: 30, Quality: 80, Mode: "extend", Position: "right"}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(current, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := loadSettingsWithLegacy(current, legacy)
			if contents == `{"resolution":"1024x768","fps":15,"quality":60}` {
				if err != nil || got != (Settings{Resolution: "1024x768", FPS: 15, Quality: 60, Mode: "extend", Position: "right"}) {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal("expected malformed/invalid current settings error")
			}
			after, _ := os.ReadFile(current)
			if string(after) != contents {
				t.Fatal("current settings overwritten")
			}
		})
	}
}

func TestInvalidLegacyDoesNotCreateSettings(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "RelevoDisplay", "settings.json")
	legacy := filepath.Join(root, "old.json")
	if err := os.WriteFile(legacy, []byte(`{"fps":999}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSettingsWithLegacy(current, legacy); err == nil {
		t.Fatal("invalid legacy accepted")
	}
	if _, err := os.Stat(current); !os.IsNotExist(err) {
		t.Fatal("created current settings", err)
	}
}

func TestMissingLegacyUsesDefaultsWithoutCreatingSettings(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "settings.json")
	got, err := loadSettingsWithLegacy(current, filepath.Join(root, "missing.json"))
	if err != nil || got != DefaultSettings() {
		t.Fatal(got, err)
	}
	if _, err := os.Stat(current); !os.IsNotExist(err) {
		t.Fatal("unexpected settings file", err)
	}
}

func TestOldSettingsGainModePositionDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if e := os.WriteFile(p, []byte(`{"resolution":"1024x768","fps":15,"quality":60}`), 0600); e != nil {
		t.Fatal(e)
	}
	s, e := loadSettings(p)
	if e != nil || s.Mode != "extend" || s.Position != "right" || s.KeepAwake {
		t.Fatal(s, e)
	}
	s.Mode = "desktop"
	s.Position = "above"
	s.KeepAwake = true
	if e = saveSettings(p, s); e != nil {
		t.Fatal(e)
	}
	got, e := loadSettings(p)
	if e != nil || got != s {
		t.Fatal(got, e)
	}
	for _, bad := range []Settings{{Resolution: "1024x768", FPS: 20, Quality: 70, Mode: "bad", Position: "right"}, {Resolution: "1024x768", FPS: 20, Quality: 70, Mode: "extend", Position: "bad"}} {
		if bad.Validate() == nil {
			t.Fatal("accepted", bad)
		}
	}
}

func TestHighRateAndQualitySettingsPersist(t *testing.T) {
	for _, q := range []int{60, 70, 80, 90, 95, 100} {
		s := DefaultSettings()
		s.FPS = 60
		s.Quality = q
		p := filepath.Join(t.TempDir(), "settings.json")
		if e := saveSettings(p, s); e != nil {
			t.Fatalf("60FPS Q%d: %v", q, e)
		}
		got, e := loadSettings(p)
		if e != nil || got != s {
			t.Fatal(got, e)
		}
	}
	for _, fps := range []int{0, 59, 61, 120} {
		s := DefaultSettings()
		s.FPS = fps
		if s.Validate() == nil {
			t.Fatalf("invalid UI FPS %d accepted", fps)
		}
	}
}
