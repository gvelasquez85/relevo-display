package wintray

import "testing"

func TestConnectionSettingsKeepWorkingDefaults(t *testing.T) {
	s := DefaultSettings()
	if s.Receiver() != "yam" || s.Link() != "USB" {
		t.Fatal("legacy changed")
	}
	s.Client = "relevo"
	s.Transport = "WiFi"
	s.LANAddress = "192.168.1.5"
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []Settings{
		func() Settings { v := s; v.Client = "yam"; return v }(),
		func() Settings { v := s; v.LANAddress = "0.0.0.0"; return v }(),
		func() Settings { v := s; v.Transport = "Network"; return v }(),
		func() Settings { v := s; v.Resolution = "2048x1536"; return v }(),
	} {
		if bad.Validate() == nil {
			t.Fatal("unsupported connection accepted")
		}
	}
}

func TestNativeGesturePreferenceRoundTrip(t *testing.T) {
	p := t.TempDir() + "/settings.json"
	s := DefaultSettings()
	s.NativeGestures = true
	if err := saveSettings(p, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSettings(p)
	if err != nil || !loaded.NativeGestures {
		t.Fatal("native gesture choice not persisted", err)
	}
}
