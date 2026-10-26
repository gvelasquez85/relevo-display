package yaminput

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"relevo/internal/remoteinput"
)

// keyboardDecoder belongs to one serialized connection consumer. It retains
// only held HID usages, never typed text or an input history.
type keyboardDecoder struct {
	held map[uint16]string
	refs map[string]int
}

func (d *keyboardDecoder) Reset() { d.held = nil; d.refs = nil }
func (d *keyboardDecoder) Decode(event map[string]interface{}) ([]remoteinput.Key, bool, error) {
	name, _ := event["eventName"].(string)
	switch name {
	case "vKeyboardSourceEvent":
		return nil, false, nil // Input language/configuration, not a key.
	case "KeyboardPressEvent":
		return d.decodeHID(event)
	case "KeyboardEvent", "vKeyboardEvent":
		text, ok := event["inputString"].(string)
		if !ok || len(text) > 4096 || !utf8.ValidString(text) {
			return nil, true, fmt.Errorf("invalid remote keyboard text")
		}
		if name == "vKeyboardEvent" {
			keys, err := committedKeyboardText(text)
			return keys, true, err
		}
		mods, ok := keyboardModifiers(event["modifiers"])
		if !ok {
			return nil, false, nil
		}
		for _, code := range []string{"Control", "Alt", "Shift"} {
			if d.refs[code] > 0 {
				mods[code] = true
			}
		}
		key, special := keyboardShortcutKey(text)
		if special || len(mods) > 0 {
			if key == "" {
				return nil, false, nil
			}
			if d.refs[key] > 0 {
				return nil, false, nil
			}
			var keys []remoteinput.Key
			var released []string
			for _, code := range []string{"Control", "Alt", "Shift"} {
				if mods[code] && d.refs[code] == 0 {
					keys = append(keys, remoteinput.Key{Kind: "down", Code: code})
					released = append(released, code)
				}
			}
			keys = append(keys, remoteinput.Key{Kind: "press", Code: key})
			for i := len(released) - 1; i >= 0; i-- {
				keys = append(keys, remoteinput.Key{Kind: "up", Code: released[i]})
			}
			return keys, true, nil
		}
		keys, err := committedKeyboardText(text)
		return keys, true, err
	default:
		return nil, false, nil
	}
}
func (d *keyboardDecoder) decodeHID(event map[string]interface{}) ([]remoteinput.Key, bool, error) {
	usage, ok := keyboardHIDUsage(event["keyCode"])
	pressed, valid := event["pressed"].(bool)
	if !ok || !valid {
		return nil, true, fmt.Errorf("invalid remote HID keyboard event")
	}
	code, ok := keyboardHIDCode(usage)
	if !ok {
		return nil, false, nil
	}
	if pressed {
		if _, exists := d.held[usage]; exists {
			return nil, true, nil
		}
		if d.held == nil {
			d.held = map[uint16]string{}
			d.refs = map[string]int{}
		}
		d.held[usage] = code
		d.refs[code]++
		if d.refs[code] > 1 {
			return nil, true, nil
		}
		return []remoteinput.Key{{Kind: "down", Code: code}}, true, nil
	}
	held, exists := d.held[usage]
	if !exists {
		return nil, true, nil
	}
	delete(d.held, usage)
	d.refs[held]--
	if d.refs[held] > 0 {
		return nil, true, nil
	}
	delete(d.refs, held)
	return []remoteinput.Key{{Kind: "up", Code: held}}, true, nil
}
func keyboardHIDUsage(value interface{}) (uint16, bool) {
	var n float64
	switch v := value.(type) {
	case uint:
		if v > 65535 {
			return 0, false
		}
		return uint16(v), true
	case uint8:
		return uint16(v), true
	case uint16:
		return v, true
	case uint32:
		if v > 65535 {
			return 0, false
		}
		return uint16(v), true
	case uint64:
		if v > 65535 {
			return 0, false
		}
		return uint16(v), true
	case int:
		if v < 0 || v > 65535 {
			return 0, false
		}
		return uint16(v), true
	case int8:
		if v < 0 {
			return 0, false
		}
		return uint16(v), true
	case int16:
		if v < 0 {
			return 0, false
		}
		return uint16(v), true
	case int32:
		if v < 0 || v > 65535 {
			return 0, false
		}
		return uint16(v), true
	case int64:
		if v < 0 || v > 65535 {
			return 0, false
		}
		return uint16(v), true
	case float32:
		n = float64(v)
	case float64:
		n = v
	default:
		return 0, false
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 65535 || math.Trunc(n) != n {
		return 0, false
	}
	return uint16(n), true
}
func keyboardHIDCode(usage uint16) (string, bool) {
	if usage >= 4 && usage <= 29 {
		return "Key" + string(rune('A'+usage-4)), true
	}
	if usage >= 30 && usage <= 38 {
		return "Digit" + strconv.Itoa(int(usage-29)), true
	}
	if usage == 39 {
		return "Digit0", true
	}
	if usage >= 58 && usage <= 69 {
		return "F" + strconv.Itoa(int(usage-57)), true
	}
	if usage >= 104 && usage <= 115 {
		return "F" + strconv.Itoa(int(usage-91)), true
	}
	if usage >= 89 && usage <= 97 {
		return "Numpad" + strconv.Itoa(int(usage-88)), true
	}
	code, ok := keyboardHIDCodes[usage]
	return code, ok
}

var keyboardHIDCodes = map[uint16]string{
	40: "Enter", 41: "Escape", 42: "Backspace", 43: "Tab", 44: "Space", 45: "Minus", 46: "Equal", 47: "BracketLeft", 48: "BracketRight", 49: "Backslash", 51: "Semicolon", 52: "Quote", 53: "Backquote", 54: "Comma", 55: "Period", 56: "Slash", 57: "CapsLock",
	70: "PrintScreen", 71: "ScrollLock", 72: "Pause", 73: "Insert", 74: "Home", 75: "PageUp", 76: "Delete", 77: "End", 78: "PageDown", 79: "ArrowRight", 80: "ArrowLeft", 81: "ArrowDown", 82: "ArrowUp", 83: "NumLock", 84: "NumpadDivide", 85: "NumpadMultiply", 86: "NumpadSubtract", 87: "NumpadAdd", 88: "NumpadEnter", 98: "Numpad0", 99: "NumpadDecimal", 100: "IntlBackslash", 101: "ContextMenu",
	// iPad Command maps to Windows Control, matching the compatible shortcut
	// vocabulary; left/right and Command/Control share reference-counted releases.
	0xe0: "Control", 0xe1: "Shift", 0xe2: "Alt", 0xe3: "Control", 0xe4: "Control", 0xe5: "Shift", 0xe6: "Alt", 0xe7: "Control",
}

func keyboardModifiers(value interface{}) (map[string]bool, bool) {
	mods := map[string]bool{}
	if value == nil {
		return mods, true
	}
	text, ok := value.(string)
	if !ok || len(text) > 128 {
		return nil, false
	}
	text = strings.NewReplacer("⌥", " ALT ", "⌘", " CMD ").Replace(strings.ToUpper(text))
	fields := strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) || r == ',' || r == '+' || r == '|' })
	for _, token := range fields {
		switch token {
		case "ALT":
			mods["Alt"] = true
		case "CMD", "CTRL":
			mods["Control"] = true
		case "SHIFT":
			mods["Shift"] = true
		case "⇪": // Committed text already reflects Caps Lock; no extra Shift.
		default:
			return nil, false
		}
	}
	return mods, true
}
func keyboardShortcutKey(text string) (code string, special bool) {
	switch text {
	case "DEL", "\b", "\x7f":
		return "Backspace", true
	case "TAB", "\t":
		return "Tab", true
	case "ENTER", "\r", "\n":
		return "Enter", true
	case "ESC", "\x1b":
		return "Escape", true
	case "↑":
		return "ArrowUp", true
	case "↓":
		return "ArrowDown", true
	case "←":
		return "ArrowLeft", true
	case "→":
		return "ArrowRight", true
	case " ":
		return "Space", true
	}
	if len(text) != 1 {
		return "", false
	}
	c := text[0]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	if c >= 'A' && c <= 'Z' {
		return "Key" + string(c), false
	}
	if c >= '0' && c <= '9' {
		return "Digit" + string(c), false
	}
	code = map[byte]string{'-': "Minus", '=': "Equal", '[': "BracketLeft", ']': "BracketRight", '\\': "Backslash", ';': "Semicolon", '\'': "Quote", '`': "Backquote", ',': "Comma", '.': "Period", '/': "Slash"}[c]
	return code, false
}
func committedKeyboardText(text string) ([]remoteinput.Key, error) {
	if len(text) > 4096 || !utf8.ValidString(text) {
		return nil, fmt.Errorf("invalid committed keyboard text")
	}
	var keys []remoteinput.Key
	var run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			keys = append(keys, remoteinput.Key{Kind: "text", Text: run.String()})
			run.Reset()
		}
	}
	previousCR := false
	for _, r := range text {
		switch r {
		case '\r':
			flush()
			keys = append(keys, remoteinput.Key{Kind: "press", Code: "Enter"})
			previousCR = true
		case '\n':
			flush()
			if !previousCR {
				keys = append(keys, remoteinput.Key{Kind: "press", Code: "Enter"})
			}
			previousCR = false
		case '\t':
			flush()
			keys = append(keys, remoteinput.Key{Kind: "press", Code: "Tab"})
			previousCR = false
		case '\b', 127:
			flush()
			keys = append(keys, remoteinput.Key{Kind: "press", Code: "Backspace"})
			previousCR = false
		default:
			if unicode.IsControl(r) {
				return nil, fmt.Errorf("unsupported committed keyboard control character")
			}
			run.WriteRune(r)
			previousCR = false
		}
	}
	flush()
	return keys, nil
}

// Held reports a logical key without exposing the raw HID history.
func (d *keyboardDecoder) Held(code string) bool { return d.refs[code] > 0 }
