// Package remoteinput validates the next negotiated Relevo input extension.
// It does not inject input or enable capabilities in existing sessions.
package remoteinput

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"unicode/utf8"
)

const MaxContacts = 10

type Contact struct {
	ID    uint32  `json:"id"`
	Phase string  `json:"phase"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

// State belongs to one connection and is used by its serialized input consumer.
type State struct{ active map[uint32]Contact }

// Apply validates a batch atomically so malformed input cannot leave contacts down.
// Unchanged active contacts retain their previous coordinates.
func (s *State) Apply(batch []Contact) error {
	if len(batch) == 0 || len(batch) > MaxContacts {
		return fmt.Errorf("invalid contact count")
	}
	next := make(map[uint32]Contact, len(s.active))
	for id, c := range s.active {
		next[id] = c
	}
	seen := make(map[uint32]bool, len(batch))
	for _, c := range batch {
		if c.ID == 0 || seen[c.ID] || math.IsNaN(c.X) || math.IsNaN(c.Y) || math.IsInf(c.X, 0) || math.IsInf(c.Y, 0) || c.X < 0 || c.X > 1 || c.Y < 0 || c.Y > 1 {
			return fmt.Errorf("invalid contact")
		}
		seen[c.ID] = true
		_, exists := next[c.ID]
		switch c.Phase {
		case "down":
			if exists {
				return fmt.Errorf("contact already down")
			}
			next[c.ID] = c
		case "move":
			if !exists {
				return fmt.Errorf("contact not down")
			}
			next[c.ID] = c
		case "up", "cancel":
			if !exists {
				return fmt.Errorf("contact not down")
			}
			delete(next, c.ID)
		default:
			return fmt.Errorf("invalid contact phase")
		}
	}
	if len(next) > MaxContacts {
		return fmt.Errorf("too many active contacts")
	}
	s.active = next
	return nil
}

// Cancel produces releases in a stable order, then clears connection state.
func (s *State) Cancel() []Contact {
	out := make([]Contact, 0, len(s.active))
	for _, c := range s.active {
		c.Phase = "cancel"
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	s.active = nil
	return out
}

type Key struct {
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
	Code string `json:"code,omitempty"`
}

// Text is committed Unicode, not intermediate IME composition or a key log.
func (k Key) Validate() error {
	if k.Kind == "text" {
		if k.Code != "" || k.Text == "" || len(k.Text) > 4096 || !utf8.ValidString(k.Text) {
			return fmt.Errorf("invalid committed text")
		}
		for _, r := range k.Text {
			if r < 32 || r == 127 {
				return fmt.Errorf("control character must use a key event")
			}
		}
		return nil
	}
	if k.Text != "" || (k.Kind != "down" && k.Kind != "up" && k.Kind != "press") {
		return fmt.Errorf("invalid keyboard event")
	}
	if len(k.Code) == 4 && k.Code[:3] == "Key" && k.Code[3] >= 'A' && k.Code[3] <= 'Z' {
		return nil
	}
	if len(k.Code) == 6 && k.Code[:5] == "Digit" && k.Code[5] >= '0' && k.Code[5] <= '9' {
		return nil
	}
	if len(k.Code) >= 2 && k.Code[0] == 'F' {
		n, e := strconv.Atoi(k.Code[1:])
		if e == nil && n >= 1 && n <= 24 && k.Code == "F"+strconv.Itoa(n) {
			return nil
		}
	}
	switch k.Code {
	case "CapsLock", "NumLock", "ScrollLock", "PrintScreen", "Pause", "Insert", "ContextMenu", "Minus", "Equal", "BracketLeft", "BracketRight", "Backslash", "IntlBackslash", "Semicolon", "Quote", "Backquote", "Comma", "Period", "Slash", "Numpad0", "Numpad1", "Numpad2", "Numpad3", "Numpad4", "Numpad5", "Numpad6", "Numpad7", "Numpad8", "Numpad9", "NumpadDivide", "NumpadMultiply", "NumpadSubtract", "NumpadAdd", "NumpadEnter", "NumpadDecimal", "Space", "Control", "Alt", "Shift", "Meta", "Enter", "Tab", "Escape", "Backspace", "Delete", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End", "PageUp", "PageDown":
		return nil
	}
	return fmt.Errorf("unsupported key code")
}

// KeyboardState tracks held keys for disconnect and mode-change cleanup.
type KeyboardState struct{ held map[string]bool }

func (s *KeyboardState) Apply(k Key) error {
	if err := k.Validate(); err != nil {
		return err
	}
	switch k.Kind {
	case "down":
		if s.held == nil {
			s.held = make(map[string]bool)
		}
		s.held[k.Code] = true
	case "up":
		delete(s.held, k.Code)
	case "press":
		if s.held[k.Code] {
			return fmt.Errorf("cannot press an already held key")
		}
	}
	return nil
}

func (s *KeyboardState) Release() []Key {
	keys := make([]Key, 0, len(s.held))
	for code := range s.held {
		keys = append(keys, Key{Kind: "up", Code: code})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Code < keys[j].Code })
	s.held = nil
	return keys
}
