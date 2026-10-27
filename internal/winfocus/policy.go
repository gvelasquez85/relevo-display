// Package winfocus detects a narrow set of native editable Windows controls
// without reading their text, accessibility values, or password contents.
package winfocus

import "strings"

type metadata struct {
	class                                 string
	enabled, readOnly, active, hitFocused bool
}

func editable(m metadata) bool {
	if !m.enabled || m.readOnly || !m.active || !m.hitFocused {
		return false
	}
	switch strings.ToLower(m.class) {
	case "edit", "richedit", "richedit20a", "richedit20w", "richedit50a", "richedit50w", "richeditd2dpt", "richeditd2d":
		return true
	}
	return false
}

// automationMetadata contains only interaction metadata, never field contents.
type automationMetadata struct {
	controlType                                                              int32
	enabled, focused, focusable, readOnlyKnown, readOnly, active, hitFocused bool
}

func automationEditable(m automationMetadata) bool {
	return m.controlType == 50004 && m.enabled && m.focused && m.focusable && m.readOnlyKnown && !m.readOnly && m.active && m.hitFocused
}
