package remoteinput

import "testing"

func TestExtendedKeyboardCodes(t *testing.T) {
	for _, code := range []string{"Minus", "Equal", "BracketLeft", "BracketRight", "Backslash", "IntlBackslash", "Semicolon", "Quote", "Backquote", "Comma", "Period", "Slash", "CapsLock", "NumLock", "ScrollLock", "PrintScreen", "Pause", "Insert", "ContextMenu", "Numpad0", "Numpad9", "NumpadDivide", "NumpadMultiply", "NumpadSubtract", "NumpadAdd", "NumpadDecimal", "NumpadEnter", "F13", "F24"} {
		if err := (Key{Kind: "press", Code: code}).Validate(); err != nil {
			t.Fatal("extended code rejected", code)
		}
	}
	for _, code := range []string{"F25", "F013", "Numpad10", "Numpad-1", "NumpadEquals", "KeyPrivate", "CapsLockExtra"} {
		if (Key{Kind: "press", Code: code}).Validate() == nil {
			t.Fatal("invalid extended key accepted")
		}
	}
}
