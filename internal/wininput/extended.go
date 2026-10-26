package wininput

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
	"unicode/utf16"

	"relevo/internal/remoteinput"
)

// TouchPacket uses physical desktop pixels; IDs are bounded local slots (1..10).
// Touch receives a complete frame, including unchanged contacts as moves.
type TouchPacket struct {
	ID    uint32
	Phase string
	X, Y  int32
}
type KeyPacket struct {
	VK, Scan              uint16
	Unicode, Extended, Up bool
}

// Backend calls must synchronously consume the provided slices. Keyboard must
// attempt to release partially submitted key batches before returning an error.
type ExtendedBackend interface {
	Touch([]TouchPacket) error
	Keyboard([]KeyPacket) error
}
type ExtendedController struct {
	mu             sync.Mutex
	target         func() (Target, error)
	backend        ExtendedBackend
	wire           remoteinput.State
	active         map[uint32]TouchPacket
	held           map[string]KeyPacket
	pinned         Target
	closed         bool
	failure        error
	releasePending bool
	done           chan struct{}
}

func NewExtended(target func() (Target, error), backend ExtendedBackend) *ExtendedController {
	c := &ExtendedController{target: target, backend: backend, active: make(map[uint32]TouchPacket), held: make(map[string]KeyPacket), done: make(chan struct{})}
	go c.maintain()
	return c
}

// Windows needs stationary UPDATE frames for press-and-hold. This also detects
// a lost/changed target while the user's fingers remain stationary.
func (c *ExtendedController) maintain() {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-tick.C:
			c.mu.Lock()
			if !c.closed && c.failure != nil && c.releasePending {
				// A secure desktop can temporarily reject releases. Retry only releases,
				// never the original action, until the pending state is cleared.
				_ = c.release()
			} else if !c.closed && c.failure == nil && (len(c.active) > 0 || len(c.held) > 0) {
				t, e := c.target()
				if e == nil && t != c.pinned {
					e = fmt.Errorf("touch target changed")
				}
				if e == nil && len(c.active) > 0 {
					e = c.backend.Touch(c.frame(nil))
				}
				if e != nil {
					c.failure = errors.Join(e, c.release())
				}
			}
			c.mu.Unlock()
		}
	}
}
func (c *ExtendedController) frame(changes map[uint32]TouchPacket) []TouchPacket {
	out := make([]TouchPacket, 0, len(c.active))
	for id, p := range c.active {
		if q, ok := changes[id]; ok {
			p = q
		} else {
			p.Phase = "move"
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (c *ExtendedController) release() error {
	var result error
	if len(c.active) > 0 {
		changes := make(map[uint32]TouchPacket, len(c.active))
		for id, p := range c.active {
			p.Phase = "cancel"
			changes[id] = p
		}
		e := c.backend.Touch(c.frame(changes))
		result = errors.Join(result, e)
		if e == nil {
			clear(c.active)
		}
	}
	c.wire.Cancel()
	codes := make([]string, 0, len(c.held))
	for code := range c.held {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	// Attempt every release, even if an earlier one fails.
	for _, code := range codes {
		p := c.held[code]
		p.Up = true
		e := c.backend.Keyboard([]KeyPacket{p})
		result = errors.Join(result, e)
		if e == nil {
			delete(c.held, code)
		}
	}
	if pending, ok := c.backend.(interface{ Release() error }); ok {
		result = errors.Join(result, pending.Release())
	}
	c.releasePending = result != nil
	return result
}
func (c *ExtendedController) fail(e error) error {
	c.failure = errors.Join(e, c.release())
	return c.failure
}
func (c *ExtendedController) check() error {
	if c.closed {
		return fmt.Errorf("input closed")
	}
	if c.failure != nil {
		return c.failure
	}
	if c.target == nil || c.backend == nil {
		return fmt.Errorf("input not configured")
	}
	return nil
}
func pixelCoordinates(t Target, x, y float64) (int32, int32, error) {
	if _, _, e := Coordinates(t, x, y); e != nil {
		return 0, 0, e
	}
	// Do not convert through the legacy 16-bit mouse coordinate space: doing so
	// introduces rounding errors and is wrong on negative virtual origins.
	px := x * float64(t.FrameWidth-1)
	py := y * float64(t.FrameHeight-1)
	sx := float64(t.Screen.Min.X) + (px-float64(t.Content.Min.X))*float64(t.Screen.Dx()-1)/float64(t.Content.Dx()-1)
	sy := float64(t.Screen.Min.Y) + (py-float64(t.Content.Min.Y))*float64(t.Screen.Dy()-1)/float64(t.Content.Dy()-1)
	if sx < math.MinInt32 || sx > math.MaxInt32 || sy < math.MinInt32 || sy > math.MaxInt32 {
		return 0, 0, fmt.Errorf("desktop coordinates overflow")
	}
	return int32(math.Round(sx)), int32(math.Round(sy)), nil
}
func (c *ExtendedController) HandleContacts(batch []remoteinput.Contact) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(); e != nil {
		return e
	}
	if e := c.wire.Apply(batch); e != nil {
		return c.fail(e)
	}
	t, e := c.target()
	if e != nil {
		return c.fail(e)
	}
	if (len(c.active) > 0 || len(c.held) > 0) && t != c.pinned {
		return c.fail(fmt.Errorf("touch target changed"))
	}
	c.pinned = t
	// Replacement contacts cannot share a frame with more than ten previous
	// contacts. Release old slots first, then submit the new contacts. This also
	// handles a finger leaving the letterbox while another finger starts.
	hasDown := false
	for _, event := range batch {
		if event.Phase == "down" {
			hasDown = true
		}
	}
	if hasDown && len(c.active) > 0 {
		ends := make(map[uint32]TouchPacket)
		for _, event := range batch {
			p, exists := c.active[event.ID]
			if !exists {
				continue
			}
			if event.Phase == "up" || event.Phase == "cancel" {
				p.Phase = event.Phase
				ends[event.ID] = p
				continue
			}
			if event.Phase == "move" {
				_, _, e := pixelCoordinates(t, event.X, event.Y)
				if errors.Is(e, ErrOutsideContent) {
					p.Phase = "cancel"
					ends[event.ID] = p
				} else if e != nil {
					return c.fail(e)
				}
			}
		}
		if len(ends) > 0 {
			if e := c.backend.Touch(c.frame(ends)); e != nil {
				return c.fail(e)
			}
			for id := range ends {
				delete(c.active, id)
			}
		}
	}
	changes := make(map[uint32]TouchPacket, len(batch))
	for _, event := range batch {
		p, exists := c.active[event.ID]
		if event.Phase == "cancel" || event.Phase == "up" {
			if exists {
				p.Phase = event.Phase
				changes[event.ID] = p
			}
			continue
		}
		x, y, err := pixelCoordinates(t, event.X, event.Y)
		if errors.Is(err, ErrOutsideContent) {
			if exists {
				p.Phase = "cancel"
				changes[event.ID] = p
			}
			continue
		}
		if err != nil {
			return c.fail(err)
		}
		if event.Phase == "down" {
			used := make(map[uint32]bool)
			for _, p := range c.active {
				used[p.ID] = true
			}
			for slot := uint32(1); slot <= remoteinput.MaxContacts; slot++ {
				if !used[slot] {
					p.ID = slot
					break
				}
			}
			if p.ID == 0 {
				return c.fail(fmt.Errorf("no free touch slot"))
			}
		} else if !exists {
			continue
		} // Contact started in/left a black letterbox band.
		p.X, p.Y, p.Phase = x, y, event.Phase
		c.active[event.ID] = p
		changes[event.ID] = p
	}
	frame := c.frame(changes)
	if len(frame) > 0 {
		if e := c.backend.Touch(frame); e != nil {
			return c.fail(e)
		}
	}
	for id, p := range changes {
		if p.Phase == "up" || p.Phase == "cancel" {
			delete(c.active, id)
		}
	}
	return nil
}

var virtualKeys = map[string]KeyPacket{
	"CapsLock": {VK: 0x14}, "NumLock": {VK: 0x90, Extended: true}, "ScrollLock": {VK: 0x91}, "PrintScreen": {VK: 0x2c, Extended: true}, "Pause": {VK: 0x13}, "Insert": {VK: 0x2d, Extended: true}, "ContextMenu": {VK: 0x5d, Extended: true},
	"Minus": {VK: 0xbd}, "Equal": {VK: 0xbb}, "BracketLeft": {VK: 0xdb}, "BracketRight": {VK: 0xdd}, "Backslash": {VK: 0xdc}, "IntlBackslash": {VK: 0xe2}, "Semicolon": {VK: 0xba}, "Quote": {VK: 0xde}, "Backquote": {VK: 0xc0}, "Comma": {VK: 0xbc}, "Period": {VK: 0xbe}, "Slash": {VK: 0xbf},
	"Numpad0": {VK: 0x60}, "Numpad1": {VK: 0x61}, "Numpad2": {VK: 0x62}, "Numpad3": {VK: 0x63}, "Numpad4": {VK: 0x64}, "Numpad5": {VK: 0x65}, "Numpad6": {VK: 0x66}, "Numpad7": {VK: 0x67}, "Numpad8": {VK: 0x68}, "Numpad9": {VK: 0x69}, "NumpadMultiply": {VK: 0x6a}, "NumpadAdd": {VK: 0x6b}, "NumpadSubtract": {VK: 0x6d}, "NumpadDecimal": {VK: 0x6e}, "NumpadDivide": {VK: 0x6f, Extended: true}, "NumpadEnter": {VK: 0x0d, Extended: true},

	"Control": {VK: 0x11}, "Alt": {VK: 0x12}, "Shift": {VK: 0x10}, "Meta": {VK: 0x5b, Extended: true},
	"Enter": {VK: 0x0d}, "Tab": {VK: 9}, "Escape": {VK: 0x1b}, "Backspace": {VK: 8},
	"Delete": {VK: 0x2e, Extended: true}, "ArrowLeft": {VK: 0x25, Extended: true}, "ArrowRight": {VK: 0x27, Extended: true}, "ArrowUp": {VK: 0x26, Extended: true}, "ArrowDown": {VK: 0x28, Extended: true},
	"Home": {VK: 0x24, Extended: true}, "End": {VK: 0x23, Extended: true}, "PageUp": {VK: 0x21, Extended: true}, "PageDown": {VK: 0x22, Extended: true},
}

func (c *ExtendedController) HandleKey(k remoteinput.Key) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(); e != nil {
		return e
	}
	if e := k.Validate(); e != nil {
		return c.fail(e)
	}
	// A key is never forwarded to a secure desktop or detached/stale display.
	t, e := c.target()
	if e != nil {
		return c.fail(e)
	}
	if _, _, e = pixelCoordinates(t, .5, .5); e != nil {
		return c.fail(e)
	}
	if (len(c.active) > 0 || len(c.held) > 0) && t != c.pinned {
		return c.fail(fmt.Errorf("touch target changed"))
	}
	c.pinned = t
	var packets []KeyPacket
	if k.Kind == "text" {
		units := utf16.Encode([]rune(k.Text))
		packets = make([]KeyPacket, 0, 2*len(units))
		for _, u := range units {
			packets = append(packets, KeyPacket{Scan: u, Unicode: true}, KeyPacket{Scan: u, Unicode: true, Up: true})
		}
	} else {
		p, ok := lookupVirtualKey(k.Code)
		if !ok {
			return c.fail(fmt.Errorf("unsupported virtual key"))
		}
		_, held := c.held[k.Code]
		switch k.Kind {
		case "down":
			if held {
				return nil
			}
			c.held[k.Code] = p
			packets = []KeyPacket{p}
		case "up":
			if !held {
				return nil
			}
			p.Up = true
			packets = []KeyPacket{p}
		case "press":
			if held {
				return c.fail(fmt.Errorf("key already held"))
			}
			c.held[k.Code] = p
			up := p
			up.Up = true
			packets = []KeyPacket{p, up}
		}
	}
	if e := c.backend.Keyboard(packets); e != nil {
		return c.fail(e)
	}
	if k.Kind == "up" || k.Kind == "press" {
		delete(c.held, k.Code)
	}
	return nil
}
func (c *ExtendedController) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.done)
	}
	return c.release()
}

func lookupVirtualKey(code string) (KeyPacket, bool) {
	if p, ok := virtualKeys[code]; ok {
		return p, true
	}
	if code == "Space" {
		return KeyPacket{VK: 0x20}, true
	}
	if len(code) == 4 && code[:3] == "Key" && code[3] >= 'A' && code[3] <= 'Z' {
		return KeyPacket{VK: uint16(code[3])}, true
	}
	if len(code) == 6 && code[:5] == "Digit" && code[5] >= '0' && code[5] <= '9' {
		return KeyPacket{VK: uint16(code[5])}, true
	}
	if len(code) >= 2 && code[0] == 'F' {
		n, e := strconv.Atoi(code[1:])
		if e == nil && n >= 1 && n <= 24 && code == "F"+strconv.Itoa(n) {
			return KeyPacket{VK: uint16(0x6f + n)}, true
		}
	}
	return KeyPacket{}, false
}
