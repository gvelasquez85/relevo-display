package wininput

// Exact Windows x64 layouts. Keeping these definitions platform independent
// lets the portable tests verify sizes and offsets without calling an OS API.
type winPoint struct{ X, Y int32 }
type winRect struct{ Left, Top, Right, Bottom int32 }
type winPointerInfo struct {
	Type, ID, Frame, Flags                 uint32
	Source, Target                         uintptr
	Pixel, Himetric, PixelRaw, HimetricRaw winPoint
	Time, History                          uint32
	InputData                              int32
	KeyStates                              uint32
	Performance                            uint64
	ButtonChange                           uint32
}
type winTouchInfo struct {
	Pointer               winPointerInfo
	Flags, Mask           uint32
	Contact, ContactRaw   winRect
	Orientation, Pressure uint32
}
type winKeyboardInput struct {
	Kind, Pad         uint32
	VK, Scan          uint16
	Flags, Time, Pad2 uint32
	Extra             uintptr
	UnionTail         [8]byte
}

func touchInfo(p TouchPacket) winTouchInfo {
	t := winTouchInfo{}
	t.Pointer.Type = 2
	t.Pointer.ID = p.ID
	t.Pointer.Pixel = winPoint{p.X, p.Y}
	switch p.Phase {
	case "down":
		t.Pointer.Flags = 0x10000 | 2 | 4
	case "move":
		t.Pointer.Flags = 0x20000 | 2 | 4
	case "up":
		t.Pointer.Flags = 0x40000
	case "cancel":
		t.Pointer.Flags = 0x40000 | 0x8000
	}
	// Touch area/pressure/orientation intentionally omitted; Windows supplies
	// defaults. This avoids an artificial contact area extending off-screen.
	return t
}
func keyboardInfo(p KeyPacket) winKeyboardInput {
	k := winKeyboardInput{Kind: 1, VK: p.VK, Scan: p.Scan}
	if p.Unicode {
		k.Flags |= 4
	}
	if p.Extended {
		k.Flags |= 1
	}
	if p.Up {
		k.Flags |= 2
	}
	return k
}

// Outstanding downs from a possibly partial SendInput batch. Unicode surrogate
// units are tracked separately, as required by KEYEVENTF_UNICODE.
func keyReleases(sent []KeyPacket) []KeyPacket {
	type id struct {
		vk, scan          uint16
		unicode, extended bool
	}
	held := make(map[id]KeyPacket)
	order := make([]id, 0)
	for _, p := range sent {
		k := id{p.VK, p.Scan, p.Unicode, p.Extended}
		if p.Up {
			delete(held, k)
		} else {
			if _, ok := held[k]; !ok {
				order = append(order, k)
			}
			held[k] = p
		}
	}
	out := make([]KeyPacket, 0, len(held))
	for _, k := range order {
		if p, ok := held[k]; ok {
			p.Up = true
			out = append(out, p)
			delete(held, k)
		}
	}
	return out
}
