package yamchecks

import (
	"relevo/internal/desktop"
	"relevo/internal/session"
	"strings"
	"testing"
)

func TestDisconnectRequiresReadyObservedDisconnectAndCleanup(t *testing.T) {
	r := Result{Profile: "disconnect", Stats: session.Stats{Ready: true, FramesSent: 10, EndReason: session.EndDeviceLocked}, Cleaned: true}
	if !strings.Contains(r.Summary(), "Desconexión detectada") {
		t.Fatal(r.Summary())
	}
	r.Stats.EndReason = session.EndReadFailure
	if strings.Contains(r.Summary(), "Desconexión detectada") {
		t.Fatal("malformed input is not a passing disconnect")
	}
	r.Cleaned = false
	if !strings.Contains(r.Summary(), "no confirmada") {
		t.Fatal(r.Summary())
	}
}
func TestUACNeedsActualPauseAndRecovery(t *testing.T) {
	r := Result{Profile: "uac", Stats: session.Stats{Ready: true, FramesSent: 10}, Cleaned: true}
	if !strings.Contains(r.Summary(), "no observada") {
		t.Fatal(r.Summary())
	}
	r.Capture = desktop.CaptureStatus{Pauses: 1, Recoveries: 1}
	if !strings.Contains(r.Summary(), "Pausa y recuperación observadas") {
		t.Fatal(r.Summary())
	}
	r.Failure = "lost connection"
	if strings.Contains(r.Summary(), "Pausa y recuperación observadas") {
		t.Fatal("failed transport must not pass UAC")
	}
}
func TestCompareRequiresMatchingSettingsAndReportsSenderOnly(t *testing.T) {
	a := Result{Profile: "performance30", Resolution: "1024x768", Mode: "extend", Position: "right", Quality: 70, RequestedFPS: 30, Cleaned: true, Stats: session.Stats{Ready: true, FramesSent: 20, ActualFPS: 29}}
	b := a
	b.Profile = "performance60"
	b.RequestedFPS = 60
	b.Stats.ActualFPS = 48
	if !strings.Contains(Compare(a, b), "29.0") {
		t.Fatal("no comparison")
	}
	b.Quality = 80
	if Compare(a, b) != "" {
		t.Fatal("different settings compared")
	}
}
func TestEventSummaryFiltersArbitraryNames(t *testing.T) {
	s := EventSummary(map[string]int{"SingleTap": 3, "typed-password": 5, "KeyboardEvent": 2})
	if strings.Contains(s, "password") || !strings.Contains(s, "SingleTap=3") {
		t.Fatal(s)
	}
}
