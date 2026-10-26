package wininput

import (
	"relevo/internal/remoteinput"
	"testing"
)

func TestExtendedVirtualKeyMappings(t *testing.T) {
	cases := []struct {
		code string
		vk   uint16
		ext  bool
	}{{"Minus", 0xbd, false}, {"Equal", 0xbb, false}, {"BracketLeft", 0xdb, false}, {"BracketRight", 0xdd, false}, {"Backslash", 0xdc, false}, {"IntlBackslash", 0xe2, false}, {"Semicolon", 0xba, false}, {"Quote", 0xde, false}, {"Backquote", 0xc0, false}, {"Comma", 0xbc, false}, {"Period", 0xbe, false}, {"Slash", 0xbf, false}, {"CapsLock", 0x14, false}, {"NumLock", 0x90, true}, {"Insert", 0x2d, true}, {"NumpadEnter", 0x0d, true}, {"NumpadDivide", 0x6f, true}, {"Numpad0", 0x60, false}, {"Numpad9", 0x69, false}, {"F13", 0x7c, false}, {"F24", 0x87, false}}
	for _, tt := range cases {
		p, ok := lookupVirtualKey(tt.code)
		if !ok || p.VK != tt.vk || p.Extended != tt.ext || p.Unicode || p.Up {
			t.Fatalf("wrong virtual key mapping %s", tt.code)
		}
	}
}
func TestExtendedNumpadAndOEMDisconnectRelease(t *testing.T) {
	f := &extendedFake{}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	for _, code := range []string{"NumpadEnter", "BracketLeft", "F24"} {
		if err := c.HandleKey(remoteinput.Key{Kind: "down", Code: code}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if len(f.keys) != 6 {
		t.Fatal("not every added held key released")
	}
	for _, packets := range f.keys[3:] {
		if len(packets) != 1 || !packets[0].Up {
			t.Fatal("held extended key stuck after close")
		}
	}
}
