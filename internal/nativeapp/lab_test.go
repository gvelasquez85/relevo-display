package nativeapp

import (
	"relevo/internal/wintray"
	"testing"
	"time"
)

func TestGUIProfilesSelectOnlyRequestedFunctions(t *testing.T) {
	for _, tt := range []struct {
		profile, mode             string
		input, audio, mic, camera bool
	}{{"pattern", "pattern", false, false, false, false}, {"extend", "extend", true, false, false, false}, {"input", "desktop", true, false, false, false}, {"audio", "pattern", false, true, false, false}, {"microphone", "pattern", false, false, true, false}, {"camera", "pattern", false, false, false, true}} {
		c, e := configForLab(wintray.LabSettings{Profile: tt.profile, FPS: 60, Quality: 95})
		if e != nil {
			t.Fatal(e)
		}
		if c.Mode != tt.mode || c.Multitouch != tt.input || c.Keyboard != tt.input || c.Pointer != tt.input || c.Audio != tt.audio || c.Monitor != tt.mic || c.Microphone != tt.mic || c.Camera != tt.camera || c.FPS != 60 || c.Quality != 95 || c.Duration != 5*time.Minute {
			t.Fatalf("bad profile %s: %+v", tt.profile, c)
		}
	}
	if _, e := configForLab(wintray.LabSettings{Profile: "unknown", FPS: 30, Quality: 70}); e == nil {
		t.Fatal("unknown GUI test")
	}
}
