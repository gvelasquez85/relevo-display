package desktop

import (
	"reflect"
	"relevo/internal/winvirtual"
	"testing"
	"unsafe"
)

func topologyFixture() ([]ccdPath, []ccdMode, ccdSourceKey) {
	physical := ccdPath{Source: ccdSource{Adapter: ccdLUID{1, 0}, ID: 0, ModeIndex: 0}, Target: ccdTarget{Adapter: ccdLUID{1, 0}, ID: 1, ModeIndex: 1, Available: 1}, Flags: 1}
	virtual := ccdPath{Source: ccdSource{Adapter: ccdLUID{2, 0}, ID: 0, ModeIndex: ccdInvalidMode}, Target: ccdTarget{Adapter: ccdLUID{2, 0}, ID: 1, ModeIndex: ccdInvalidMode, Available: 1}}
	return []ccdPath{physical, virtual}, []ccdMode{{Type: 1, Adapter: physical.Source.Adapter, ID: 0, Data: [6]uint64{123, 456, 789}}, {Type: 2, Adapter: physical.Target.Adapter, ID: 1, Data: [6]uint64{999, 888, 777}}}, ccdKey(virtual)
}
func TestCCDNativeABI(t *testing.T) {
	cases := []struct {
		name      string
		got, want uintptr
	}{{"device header", unsafe.Sizeof(ccdDeviceHeader{}), 20}, {"source name", unsafe.Sizeof(ccdSourceName{}), 84}, {"source name offset", unsafe.Offsetof(ccdSourceName{}.Name), 20}, {"LUID", unsafe.Sizeof(ccdLUID{}), 8}, {"source", unsafe.Sizeof(ccdSource{}), 20}, {"target", unsafe.Sizeof(ccdTarget{}), 48}, {"path", unsafe.Sizeof(ccdPath{}), 72}, {"mode", unsafe.Sizeof(ccdMode{}), 64}, {"target offset", unsafe.Offsetof(ccdPath{}.Target), 20}, {"flags offset", unsafe.Offsetof(ccdPath{}.Flags), 68}, {"mode union", unsafe.Offsetof(ccdMode{}.Data), 16}, {"union alignment", unsafe.Alignof(ccdMode{}), 8}, {"refresh offset", unsafe.Offsetof(ccdTarget{}.Refresh), 28}, {"available offset", unsafe.Offsetof(ccdTarget{}.Available), 40}}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: %d != %d", c.name, c.got, c.want)
		}
	}
}
func TestCCDActivateDetachReactivatePreservesPhysical(t *testing.T) {
	p, m, k := topologyFixture()
	physical := p[0]
	originalModes := append([]ccdMode(nil), m...)
	want := winvirtual.Mode{Width: 1024, Height: 768, X: 1920, Refresh: 60, BitsPerPixel: 32, Attached: true}
	active, m2, e := ccdPlan(p, m, k, want)
	if e != nil {
		t.Fatal(e)
	}
	if active[0] != physical || !reflect.DeepEqual(m2[:len(m)], originalModes) {
		t.Fatal("physical path or mode changed")
	}
	v := active[1]
	if v.Target.ModeIndex != ccdInvalidMode || v.Target.Refresh != (ccdRational{60, 1}) || v.Target.Rotation != 1 || v.Target.Scaling != 1 || v.Flags != 1 {
		t.Fatalf("invalid attachment: %+v", v)
	}
	// Windows rejects UNSPECIFIED scan ordering with an explicit refresh rate.
	if v.Target.Refresh.Numerator != 0 && v.Target.ScanLine != 1 {
		t.Fatalf("explicit refresh requires progressive scan ordering, got %d", v.Target.ScanLine)
	}
	w, h, x, y := ccdSourceMode(m2[v.Source.ModeIndex])
	if w != 1024 || h != 768 || x != 1920 || y != 0 {
		t.Fatal("wrong source mode")
	}
	want.Attached = false
	detached, m3, e := ccdPlan(active, m2, k, want)
	if e != nil {
		t.Fatal(e)
	}
	if len(detached) != 1 || detached[0] != physical || !reflect.DeepEqual(m3[:len(m)], originalModes) {
		t.Fatal("detach changed physical configuration")
	}
	// QueryDisplayConfig includes the connected inactive target after detaching.
	inactive := v
	inactive.Flags = 0
	inactive.Source.ModeIndex = ccdInvalidMode
	inactive.Target.ModeIndex = ccdInvalidMode
	detached = append(detached, inactive)
	want.Attached = true
	want.Width = 1280
	again, againModes, e := ccdPlan(detached, m3, k, want)
	if e != nil || len(again) != 2 || again[0] != physical {
		t.Fatalf("reactivate: %v", e)
	}

	if len(againModes) != len(m3) {
		t.Fatal("reactivation appended a duplicate virtual source mode")
	}
	type modeKey struct {
		Type    uint32
		Adapter ccdLUID
		ID      uint32
	}
	seen := map[modeKey]bool{}
	for _, mode := range againModes {
		id := modeKey{mode.Type, mode.Adapter, mode.ID}
		if seen[id] {
			t.Fatal("duplicate CCD mode identifier")
		}
		seen[id] = true
	}
	if !reflect.DeepEqual(againModes[:len(m)], originalModes) {
		t.Fatal("reactivation changed physical mode")
	}
	width, _, _, _ := ccdSourceMode(againModes[again[1].Source.ModeIndex])
	if width != 1280 {
		t.Fatal("reactivation failed to replace stale virtual source mode")
	}
	if !reflect.DeepEqual(m, originalModes) {
		t.Fatal("input mutation")
	}
}
func TestCCDRejectUnsafeTopologies(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]ccdPath, []ccdMode) ([]ccdPath, []ccdMode)
	}{
		{"missing", func(p []ccdPath, m []ccdMode) ([]ccdPath, []ccdMode) { return p[:1], m }},
		{"disconnected", func(p []ccdPath, m []ccdMode) ([]ccdPath, []ccdMode) { p[1].Target.Available = 0; return p, m }},
		{"ambiguous", func(p []ccdPath, m []ccdMode) ([]ccdPath, []ccdMode) {
			q := p[1]
			q.Target.ID++
			return append(p, q), m
		}},
		{"primary", func(p []ccdPath, m []ccdMode) ([]ccdPath, []ccdMode) {
			p[1].Flags = 1
			p[1].Source.ModeIndex = 2
			return p, append(m, ccdMode{Type: 1, Adapter: p[1].Source.Adapter, ID: p[1].Source.ID})
		}},
		{"clone", func(p []ccdPath, m []ccdMode) ([]ccdPath, []ccdMode) { p[0].Source = p[1].Source; return p, m }},
		{"shared target", func(p []ccdPath, m []ccdMode) ([]ccdPath, []ccdMode) { p[0].Target = p[1].Target; return p, m }},
		{"no physical", func(p []ccdPath, m []ccdMode) ([]ccdPath, []ccdMode) { p[0].Flags = 0; return p, m }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, m, k := topologyFixture()
			p, m = tt.mutate(p, m)
			_, _, e := ccdPlan(p, m, k, winvirtual.Mode{Attached: true, Width: 1024, Height: 768, X: 1920, Refresh: 60, BitsPerPixel: 32})
			if e == nil {
				t.Fatal("unsafe topology accepted")
			}
		})
	}
}

func TestCCDRejectInvalidRequestedModes(t *testing.T) {
	tests := []struct {
		name   string
		change func(*winvirtual.Mode)
	}{
		{"zero width", func(m *winvirtual.Mode) { m.Width = 0 }},
		{"negative height", func(m *winvirtual.Mode) { m.Height = -1 }},
		{"excessive width", func(m *winvirtual.Mode) { m.Width = 16385 }},
		{"excessive height", func(m *winvirtual.Mode) { m.Height = 16385 }},
		{"missing refresh", func(m *winvirtual.Mode) { m.Refresh = 0 }},
		{"unsupported pixel depth", func(m *winvirtual.Mode) { m.BitsPerPixel = 24 }},
		{"primary position", func(m *winvirtual.Mode) { m.X = 0 }},
		{"rotation", func(m *winvirtual.Mode) { m.Orientation = 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, m, k := topologyFixture()
			want := winvirtual.Mode{Attached: true, Width: 1024, Height: 768, X: 1920, Refresh: 60, BitsPerPixel: 32}
			tt.change(&want)
			if _, _, err := ccdPlan(p, m, k, want); err == nil {
				t.Fatal("invalid requested mode accepted")
			}
		})
	}
}
