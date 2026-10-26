package remoteinput

import (
	"strings"
	"testing"
)

func TestTwoContactsMoveIndependentlyAndDisconnectCancelsBoth(t *testing.T) {
	var s State
	if e := s.Apply([]Contact{{ID: 1, Phase: "down", X: .2, Y: .3}, {ID: 2, Phase: "down", X: .8, Y: .7}}); e != nil {
		t.Fatal(e)
	}
	if e := s.Apply([]Contact{{ID: 1, Phase: "move", X: .3, Y: .4}, {ID: 2, Phase: "move", X: .7, Y: .6}}); e != nil {
		t.Fatal(e)
	}
	got := s.Cancel()
	if len(got) != 2 || got[0].ID != 1 || got[0].X != .3 || got[1].ID != 2 || got[1].X != .7 || got[0].Phase != "cancel" || got[1].Phase != "cancel" {
		t.Fatalf("bad cancellation: %+v", got)
	}
	if len(s.Cancel()) != 0 {
		t.Fatal("contacts remain after disconnect")
	}
}
func TestInvalidBatchDoesNotPartiallyPressContacts(t *testing.T) {
	var s State
	if e := s.Apply([]Contact{{ID: 1, Phase: "down", X: .5, Y: .5}, {ID: 2, Phase: "move", X: .2, Y: .2}}); e == nil {
		t.Fatal("accepted move before down")
	}
	if len(s.Cancel()) != 0 {
		t.Fatal("partially applied invalid frame")
	}
}
func TestDuplicateContactAndOutOfBoundsRejected(t *testing.T) {
	for _, batch := range [][]Contact{
		{{ID: 1, Phase: "down", X: .1, Y: .1}, {ID: 1, Phase: "down", X: .2, Y: .2}},
		{{ID: 1, Phase: "down", X: 1.1, Y: 0}},
	} {
		var s State
		if s.Apply(batch) == nil {
			t.Fatal("accepted invalid contacts")
		}
	}
}
func TestUnicodeKeyboardAndControlKeys(t *testing.T) {
	for _, k := range []Key{{Kind: "text", Text: "Español: acción 😀"}, {Kind: "down", Code: "Control"}, {Kind: "up", Code: "Control"}, {Kind: "press", Code: "Backspace"}} {
		if e := k.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	for _, k := range []Key{{Kind: "text", Text: string([]byte{0xff})}, {Kind: "text", Text: strings.Repeat("x", 4097)}, {Kind: "press", Code: "Unknown"}, {Kind: "text", Text: "hola", Code: "Enter"}} {
		if k.Validate() == nil {
			t.Fatal("accepted invalid keyboard event")
		}
	}
}

func TestDisconnectReleasesHeldModifiers(t *testing.T) {
	var s KeyboardState
	if e := s.Apply(Key{Kind: "down", Code: "Control"}); e != nil {
		t.Fatal(e)
	}
	if e := s.Apply(Key{Kind: "down", Code: "Shift"}); e != nil {
		t.Fatal(e)
	}
	keys := s.Release()
	if len(keys) != 2 || keys[0].Code != "Control" || keys[1].Code != "Shift" || keys[0].Kind != "up" || keys[1].Kind != "up" {
		t.Fatalf("stuck modifiers: %+v", keys)
	}
	if len(s.Release()) != 0 {
		t.Fatal("keys remain held")
	}
}
