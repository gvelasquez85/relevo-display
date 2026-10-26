package desktop

import "testing"

// A static desktop must still produce frames: damage-only recording makes
// the capture consumer time out after three seconds without screen changes.
func TestHyprlandCaptureDoesNotStopOnStaticDesktop(t *testing.T) {
	args := hyprRecorderArgs("relevo-test", 5)
	continuous, rate, privateOutput := false, false, false
	for i, a := range args {
		if a == "--no-damage" || a == "-D" {
			continuous = true
		}
		if a == "-r" && i+1 < len(args) && args[i+1] == "5" {
			rate = true
		}
		if a == "-f" && i+1 < len(args) && args[i+1] == "/proc/self/fd/3" {
			privateOutput = true
		}
	}
	if !continuous || !rate || !privateOutput {
		t.Fatalf("unsafe capture strategy: %v", args)
	}
}
