package main

import (
	"relevo/internal/wintray"
	"testing"
	"time"
)

func TestYamPlansPreserveDisplayAndOverrideComparisonFPS(t *testing.T) {
	base := wintray.Settings{Mode: "extend", Position: "left", Resolution: "1024x768", FPS: 20, Quality: 70}
	for _, tc := range []struct {
		profile  string
		fps      int
		duration time.Duration
	}{{"performance30", 30, 20 * time.Second}, {"performance60", 60, 20 * time.Second}, {"display", 60, 20 * time.Second}, {"disconnect", 60, 45 * time.Second}, {"uac", 60, 45 * time.Second}, {"input", 60, 45 * time.Second}} {
		got, d, e := yamPlan(base, wintray.LabSettings{Profile: tc.profile, FPS: 60, Quality: 90})
		if e != nil || got.FPS != tc.fps || got.Quality != 90 || got.Mode != base.Mode || got.Resolution != base.Resolution || got.Position != base.Position || d != tc.duration {
			t.Fatalf("%s %+v %v %v", tc.profile, got, d, e)
		}
	}
}
