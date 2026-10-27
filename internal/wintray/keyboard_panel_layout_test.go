package wintray

import (
	"testing"
	"unicode/utf8"
)

func TestKeyboardPanelLayoutAndEvents(t *testing.T) {
	for _, width := range []int{640, 768, 1024} {
		keys := keyboardPanelLayout(width, 280)
		seen := map[string]bool{}
		for _, key := range keys {
			if key.Bounds.Empty() {
				t.Fatal("empty key")
			}
			hit, ok := keyboardPanelHit(keys, key.Bounds.Min.X+1, key.Bounds.Min.Y+1)
			if !ok || hit.Label != key.Label {
				t.Fatalf("hit mismatch: %q", key.Label)
			}
			seen[key.Label] = true
			if key.Code != "close" && key.Code != "shift" {
				for _, shift := range []bool{false, true} {
					event := key.event(shift)
					if err := event.Validate(); err != nil {
						t.Fatalf("%q: %v", key.Label, err)
					}
					if !utf8.ValidString(event.Text) {
						t.Fatal("invalid unicode")
					}
				}
			}
		}
		for _, required := range []string{"ñ", "Borrar", "Intro", "Tab", "Espacio", "←", "→", "↑", "↓", "Cerrar", "Mayús"} {
			if !seen[required] {
				t.Fatalf("missing %s", required)
			}
		}
		if _, ok := keyboardPanelHit(keys, -1, 40); ok {
			t.Fatal("outside hit")
		}
	}
}
func TestKeyboardPanelShiftPreservesSpecialKeys(t *testing.T) {
	key := panelKey{Label: "ñ"}
	if key.event(true).Text != "Ñ" {
		t.Fatal("missing uppercase Ñ")
	}
	key = panelKey{Code: "Backspace"}
	if key.event(true).Code != "Backspace" {
		t.Fatal("shift changed control")
	}
}
