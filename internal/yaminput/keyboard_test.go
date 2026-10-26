package yaminput

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"relevo/internal/remoteinput"
)

func hidEvent(usage interface{}, pressed interface{}) map[string]interface{} {
	return map[string]interface{}{"eventName": "KeyboardPressEvent", "keyCode": usage, "pressed": pressed}
}
func textEvent(name, text, mods string) map[string]interface{} {
	return map[string]interface{}{"eventName": name, "inputString": text, "modifiers": mods}
}
func decodeKeys(t *testing.T, d *keyboardDecoder, event map[string]interface{}) []remoteinput.Key {
	t.Helper()
	keys, handled, err := d.Decode(event)
	if err != nil || !handled {
		t.Fatalf("expected handled keyboard event: %t %v", handled, err)
	}
	for _, key := range keys {
		if err := key.Validate(); err != nil {
			t.Fatal("decoder produced invalid key")
		}
	}
	return keys
}
func TestKeyboardUnicodeAndCommittedControls(t *testing.T) {
	d := &keyboardDecoder{}
	want := []remoteinput.Key{{Kind: "text", Text: "Español 🐱"}}
	if got := decodeKeys(t, d, textEvent("KeyboardEvent", "Español 🐱", "")); !reflect.DeepEqual(got, want) {
		t.Fatal("committed Unicode changed")
	}
	want = []remoteinput.Key{{Kind: "text", Text: "línea"}, {Kind: "press", Code: "Enter"}, {Kind: "text", Text: "dos"}, {Kind: "press", Code: "Tab"}, {Kind: "text", Text: "x"}, {Kind: "press", Code: "Backspace"}, {Kind: "press", Code: "Backspace"}}
	got := decodeKeys(t, d, textEvent("vKeyboardEvent", "línea\r\ndos\tx\b\x7f", ""))
	if !reflect.DeepEqual(got, want) {
		t.Fatal("committed control conversion changed")
	}
	for _, name := range []string{"KeyboardEvent", "vKeyboardEvent"} {
		if got := decodeKeys(t, d, textEvent(name, "", "")); len(got) != 0 {
			t.Fatal("empty input produced key")
		}
	}
}
func TestKeyboardHIDReferenceCountingAndReset(t *testing.T) {
	d := &keyboardDecoder{}
	for i, usage := range []uint16{0xe0, 0xe4, 0xe3, 0xe7} {
		got := decodeKeys(t, d, hidEvent(usage, true))
		if i == 0 {
			if !reflect.DeepEqual(got, []remoteinput.Key{{Kind: "down", Code: "Control"}}) {
				t.Fatal("missing Control down")
			}
		} else if len(got) != 0 {
			t.Fatal("duplicate logical modifier down")
		}
	}
	if len(decodeKeys(t, d, hidEvent(uint16(0xe0), true))) != 0 {
		t.Fatal("duplicate HID keydown emitted")
	}
	for i, usage := range []uint16{0xe4, 0xe0, 0xe3, 0xe7} {
		got := decodeKeys(t, d, hidEvent(usage, false))
		if i < 3 {
			if len(got) != 0 {
				t.Fatal("modifier released while another side still held")
			}
		} else if !reflect.DeepEqual(got, []remoteinput.Key{{Kind: "up", Code: "Control"}}) {
			t.Fatal("final modifier not released")
		}
	}
	if len(decodeKeys(t, d, hidEvent(uint16(0xe0), false))) != 0 {
		t.Fatal("orphan keyup emitted")
	}
	decodeKeys(t, d, hidEvent(uint64(4), true))
	d.Reset()
	if len(decodeKeys(t, d, hidEvent(uint64(4), false))) != 0 {
		t.Fatal("reset retained held usage")
	}
	if got := decodeKeys(t, d, hidEvent(uint64(4), true)); !reflect.DeepEqual(got, []remoteinput.Key{{Kind: "down", Code: "KeyA"}}) {
		t.Fatal("reset failed to permit fresh keydown")
	}
}
func TestKeyboardShortcutPreservesHeldModifiers(t *testing.T) {
	d := &keyboardDecoder{}
	decodeKeys(t, d, hidEvent(uint16(0xe0), true))
	got := decodeKeys(t, d, textEvent("KeyboardEvent", "c", "⌘ SHIFT"))
	want := []remoteinput.Key{{Kind: "down", Code: "Shift"}, {Kind: "press", Code: "KeyC"}, {Kind: "up", Code: "Shift"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("shortcut released an existing modifier")
	}
	got = decodeKeys(t, d, hidEvent(uint16(0xe0), false))
	if !reflect.DeepEqual(got, []remoteinput.Key{{Kind: "up", Code: "Control"}}) {
		t.Fatal("shortcut altered HID state")
	}
	got = decodeKeys(t, d, textEvent("KeyboardEvent", "x", "⌘,⌥,SHIFT"))
	want = []remoteinput.Key{{Kind: "down", Code: "Control"}, {Kind: "down", Code: "Alt"}, {Kind: "down", Code: "Shift"}, {Kind: "press", Code: "KeyX"}, {Kind: "up", Code: "Shift"}, {Kind: "up", Code: "Alt"}, {Kind: "up", Code: "Control"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("ephemeral shortcut order changed")
	}
	decodeKeys(t, d, hidEvent(uint16(6), true))
	if keys, handled, err := d.Decode(textEvent("KeyboardEvent", "c", "CTRL")); err != nil || handled || len(keys) != 0 {
		t.Fatal("shortcut repressed an already held letter")
	}
}
func TestKeyboardSpecialTokensAndHIDMappings(t *testing.T) {
	d := &keyboardDecoder{}
	for input, code := range map[string]string{"DEL": "Backspace", "TAB": "Tab", "ENTER": "Enter", "ESC": "Escape", "↑": "ArrowUp", "↓": "ArrowDown", "←": "ArrowLeft", "→": "ArrowRight", " ": "Space"} {
		if got := decodeKeys(t, d, textEvent("KeyboardEvent", input, "")); !reflect.DeepEqual(got, []remoteinput.Key{{Kind: "press", Code: code}}) {
			t.Fatal("special key conversion changed")
		}
	}
	for usage, code := range map[uint16]string{4: "KeyA", 29: "KeyZ", 30: "Digit1", 39: "Digit0", 58: "F1", 69: "F12", 104: "F13", 115: "F24", 45: "Minus", 49: "Backslash", 57: "CapsLock", 73: "Insert", 83: "NumLock", 88: "NumpadEnter", 97: "Numpad9", 100: "IntlBackslash", 101: "ContextMenu"} {
		got := decodeKeys(t, d, hidEvent(usage, true))
		if !reflect.DeepEqual(got, []remoteinput.Key{{Kind: "down", Code: code}}) {
			t.Fatalf("HID mapping %d changed", usage)
		}
		decodeKeys(t, d, hidEvent(usage, false))
	}
}
func TestKeyboardRejectsMalformedWithoutLeakingValues(t *testing.T) {
	for _, event := range []map[string]interface{}{
		hidEvent(-1, true), hidEvent(65536, true), hidEvent(math.NaN(), true), hidEvent(math.Inf(1), true), hidEvent(4.5, true), hidEvent("private HID", true), hidEvent(4, "private pressed"),
		textEvent("KeyboardEvent", strings.Repeat("private", 600), ""), textEvent("KeyboardEvent", string([]byte{0xff}), ""), textEvent("vKeyboardEvent", "private\x00secret", ""), textEvent("vKeyboardEvent", "private\u0085secret", ""),
	} {
		d := &keyboardDecoder{}
		keys, handled, err := d.Decode(event)
		if err == nil || !handled || len(keys) != 0 {
			t.Fatal("malformed keyboard accepted")
		}
		if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret") {
			t.Fatal("private input leaked")
		}
		if len(d.held) != 0 {
			t.Fatal("invalid event mutated held state")
		}
	}
}
func TestKeyboardUnsupportedEventsAreNonfatal(t *testing.T) {
	for _, event := range []map[string]interface{}{
		{"eventName": "SingleTap"}, {"eventName": "vKeyboardSourceEvent", "inputString": "private language"}, hidEvent(uint64(0xffff), true), hidEvent(uint64(0), true), textEvent("KeyboardEvent", "a", "unknown private modifier"), textEvent("KeyboardEvent", "é", "CTRL"),
	} {
		keys, handled, err := (&keyboardDecoder{}).Decode(event)
		if err != nil || handled || len(keys) != 0 {
			t.Fatal("unsupported keyboard event was not ignored")
		}
	}
}

func TestCommittedCapsLockTextDoesNotAddShift(t *testing.T) {
	keys, handled, err := (&keyboardDecoder{}).Decode(textEvent("KeyboardEvent", "A", "⇪"))
	if err != nil || !handled || len(keys) != 1 || keys[0].Text != "A" {
		t.Fatal("committed Caps Lock text lost")
	}
}
