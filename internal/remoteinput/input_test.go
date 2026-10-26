package remoteinput

import "testing"

func TestShortcutKeyCodes(t *testing.T) {
	for _, code := range []string{"KeyA", "KeyC", "KeyZ", "Digit0", "Digit9", "Space", "F1", "F12"} {
		if err := (Key{Kind: "press", Code: code}).Validate(); err != nil {
			t.Fatal(code, err)
		}
	}
	for _, code := range []string{"Keya", "KeyAA", "Digit10", "F0", "F13", "F01"} {
		if (Key{Kind: "press", Code: code}).Validate() == nil {
			t.Fatal("invalid code", code)
		}
	}
}
