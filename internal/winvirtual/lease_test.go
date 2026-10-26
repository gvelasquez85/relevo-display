package winvirtual

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type fakeBackend struct {
	mu       sync.Mutex
	displays []Display
	sets     []string
	failSet  bool
}

func (f *fakeBackend) List(ctx context.Context) ([]Display, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Display(nil), f.displays...), nil
}
func (f *fakeBackend) Set(ctx context.Context, name string, m Mode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets = append(f.sets, name)
	for i := range f.displays {
		if f.displays[i].Name == name {
			f.displays[i].Mode = m
		}
	}
	if f.failSet {
		f.failSet = false
		return errors.New("partial set failure")
	}
	return nil
}
func fixture() *fakeBackend {
	return &fakeBackend{displays: []Display{{Name: "physical", Primary: true, HardwareID: "real", Mode: Mode{Width: 1920, Height: 1080, Attached: true}}, {Name: "virtual", HardwareID: `root\mttvdd`, Mode: Mode{Width: 800, Height: 600, Attached: true}}}}
}
func TestAcquireRestoresOnlyVirtual(t *testing.T) {
	f := fixture()
	original := f.displays[1].Mode
	physical := f.displays[0]
	l, e := Acquire(context.Background(), f, 1280, 720)
	if e != nil {
		t.Fatal(e)
	}
	if l.DisplayName != "virtual" || l.Mode.X != 1920 || l.Mode.Width != 1280 || l.Mode.Height != 720 || l.Mode.Refresh != 60 || l.Mode.BitsPerPixel != 32 || !l.Mode.Attached {
		t.Fatalf("bad mode %+v", l)
	}
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	if f.displays[0] != physical || f.displays[1].Mode != original {
		t.Fatal("baseline not restored")
	}
	if len(f.sets) != 2 || f.sets[0] != "virtual" || f.sets[1] != "virtual" {
		t.Fatal(f.sets)
	}
}
func TestRestoreDisabled(t *testing.T) {
	f := fixture()
	f.displays[1].Mode.Attached = false
	original := f.displays[1].Mode
	l, e := Acquire(context.Background(), f, 1280, 720)
	if e != nil {
		t.Fatal(e)
	}
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	if f.displays[1].Mode != original {
		t.Fatal("disabled baseline not restored")
	}
}
func TestRejectUnsafeConfigurations(t *testing.T) {
	cases := map[string]func(*fakeBackend){"missing": func(f *fakeBackend) { f.displays = f.displays[:1] }, "ambiguous": func(f *fakeBackend) { f.displays = append(f.displays, f.displays[1]) }, "primary": func(f *fakeBackend) { f.displays[1].Primary = true }, "bad hardware": func(f *fakeBackend) { f.displays[1].HardwareID = `root\mttvdd-extra` }, "no physical": func(f *fakeBackend) { f.displays[0].Mode.Attached = false }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := fixture()
			change(f)
			if _, e := Acquire(context.Background(), f, 1280, 720); e == nil {
				t.Fatal("accepted unsafe configuration")
			}
			if len(f.sets) != 0 {
				t.Fatal("modified unsafe configuration")
			}
		})
	}
}
func TestPartialActivationFailureRollsBack(t *testing.T) {
	f := fixture()
	original := f.displays[1].Mode
	f.failSet = true
	if _, e := Acquire(context.Background(), f, 1280, 720); e == nil {
		t.Fatal("expected failure")
	}
	if f.displays[1].Mode != original || len(f.sets) != 2 {
		t.Fatal("failed to rollback")
	}
}
func TestCanceledContextDoesNotModify(t *testing.T) {
	f := fixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Acquire(ctx, f, 1280, 720); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if len(f.sets) != 0 {
		t.Fatal("modified on cancellation")
	}
}
func TestClosePreservesExternalChanges(t *testing.T) {
	f := fixture()
	l, e := Acquire(context.Background(), f, 1280, 720)
	if e != nil {
		t.Fatal(e)
	}
	f.displays[1].Mode.X = 500
	external := f.displays[1].Mode
	if e = l.Close(); e == nil {
		t.Fatal("expected external change error")
	}
	if f.displays[1].Mode != external || len(f.sets) != 1 {
		t.Fatal("clobbered external change")
	}
	if e = l.Close(); e == nil {
		t.Fatal("repeated close must retain error")
	}
}
func TestCloseRejectsChangedIdentity(t *testing.T) {
	f := fixture()
	l, e := Acquire(context.Background(), f, 1280, 720)
	if e != nil {
		t.Fatal(e)
	}
	f.displays[1].HardwareID = "different"
	if e = l.Close(); e == nil {
		t.Fatal("expected identity error")
	}
	if len(f.sets) != 1 {
		t.Fatal("modified replacement display")
	}
}

type hookedBackend struct {
	*fakeBackend
	afterSet func()
}

func (f *hookedBackend) Set(ctx context.Context, name string, m Mode) error {
	err := f.fakeBackend.Set(ctx, name, m)
	if f.afterSet != nil {
		hook := f.afterSet
		f.afterSet = nil
		hook()
	}
	return err
}
func TestCancellationAfterActivationRollsBack(t *testing.T) {
	f := fixture()
	original := f.displays[1].Mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hooked := &hookedBackend{fakeBackend: f, afterSet: cancel}
	if _, err := Acquire(ctx, hooked, 1280, 720); !errors.Is(err, context.Canceled) {
		t.Fatalf("wanted cancellation, got %v", err)
	}
	if f.displays[1].Mode != original || len(f.sets) != 2 {
		t.Fatal("canceled activation was not rolled back")
	}
}
func TestActivationFailurePreservesConcurrentChange(t *testing.T) {
	f := fixture()
	f.failSet = true
	hooked := &hookedBackend{fakeBackend: f, afterSet: func() { f.displays[1].Mode.X = 777 }}
	if _, err := Acquire(context.Background(), hooked, 1280, 720); err == nil {
		t.Fatal("wanted activation error")
	}
	if len(f.sets) != 1 || f.displays[1].Mode.X != 777 {
		t.Fatal("rollback clobbered external configuration")
	}
}
func TestCaseInsensitiveHardwareIdentity(t *testing.T) {
	f := fixture()
	f.displays[1].HardwareID = `ROOT\MTTVDD`
	l, err := Acquire(context.Background(), f, 1280, 720)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestPositionRightOfAllAttachedDisplays(t *testing.T) {
	f := fixture()
	f.displays = append(f.displays, Display{Name: "physical2", HardwareID: "real2", Mode: Mode{X: 1920, Width: 2560, Height: 1440, Attached: true}}, Display{Name: "detached", HardwareID: "real3", Mode: Mode{X: 10000, Width: 800}})
	l, err := Acquire(context.Background(), f, 1280, 720)
	if err != nil {
		t.Fatal(err)
	}
	if l.Mode.X != 4480 || l.Mode.Y != 0 {
		t.Fatalf("bad position %+v", l.Mode)
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSteppedTopologyKeepsVirtualAdjacent(t *testing.T) {
	f := fixture()
	f.displays = append(f.displays, Display{Name: "upper", HardwareID: "real2", Mode: Mode{X: 1920, Y: -1080, Width: 1920, Height: 1080, Attached: true}})
	l, err := Acquire(context.Background(), f, 1280, 720)
	if err != nil {
		t.Fatal(err)
	}
	if l.Mode.X != 3840 || l.Mode.Y != -1080 {
		t.Fatalf("not adjacent to rightmost screen: %+v", l.Mode)
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestRejectInvalidDesktopBounds(t *testing.T) {
	for name, m := range map[string]Mode{"empty": {Width: 0, Height: 1080, Attached: true}, "negative": {Width: 1920, Height: -1, Attached: true}, "overflowX": {X: 2147483640, Width: 1920, Height: 1080, Attached: true}, "overflowY": {Y: 2147483640, Width: 1920, Height: 1080, Attached: true}, "newOverflowY": {Y: 2147483000, Width: 1920, Height: 100, Attached: true}} {
		t.Run(name, func(t *testing.T) {
			f := fixture()
			f.displays[0].Mode = m
			if _, err := Acquire(context.Background(), f, 1280, 720); err == nil {
				t.Fatal("accepted invalid desktop bounds")
			}
			if len(f.sets) != 0 {
				t.Fatal("modified invalid desktop")
			}
		})
	}
}
func mode(w, h int32) Mode { return Mode{Width: w, Height: h, Refresh: 60, BitsPerPixel: 32} }
func TestChooseCaptureMode(t *testing.T) {
	tests := []struct {
		name  string
		modes []Mode
		w, h  int32
		want  Mode
		fail  bool
	}{
		{name: "exact", modes: []Mode{mode(1920, 1080), mode(1280, 720)}, w: 1280, h: 720, want: mode(1280, 720)},
		{name: "pinned driver", modes: []Mode{mode(800, 600), mode(1366, 768), mode(1920, 1080), mode(2560, 1440), mode(3840, 2160)}, w: 1280, h: 720, want: mode(1920, 1080)},
		{name: "smallest larger", modes: []Mode{mode(3840, 2160), mode(2560, 1440), mode(1920, 1080)}, w: 1280, h: 720, want: mode(1920, 1080)},
		{name: "nearest aspect", modes: []Mode{mode(800, 600), mode(1366, 768)}, w: 1280, h: 720, want: mode(1366, 768)},
		{name: "smaller compatible", modes: []Mode{mode(640, 360), mode(800, 600), mode(960, 540)}, w: 1280, h: 720, want: mode(960, 540)},
		{name: "no modes", w: 1280, h: 720, fail: true},
		{name: "invalid request", modes: []Mode{mode(1920, 1080)}, w: 0, h: 720, fail: true},
		{name: "filters", modes: []Mode{{Width: 1280, Height: 720, Refresh: 59, BitsPerPixel: 32}, {Width: 1280, Height: 720, Refresh: 60, BitsPerPixel: 24}, {Width: 1280, Height: 720, Refresh: 60, BitsPerPixel: 32, Orientation: 1}, mode(20000, 20000), mode(-1, 720), mode(1920, 1080)}, w: 1280, h: 720, want: mode(1920, 1080)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ChooseCaptureMode(tt.modes, tt.w, tt.h)
			if (err != nil) != tt.fail {
				t.Fatalf("error=%v", err)
			}
			if !tt.fail && got != tt.want {
				t.Fatalf("got %+v want %+v", got, tt.want)
			}
		})
	}
}

func TestChooseCaptureModeRejectsOnlyIncompatibleModes(t *testing.T) {
	if _, err := ChooseCaptureMode([]Mode{{Width: 1920, Height: 1080, Refresh: 75, BitsPerPixel: 32}, mode(16385, 1080)}, 1280, 720); err == nil {
		t.Fatal("accepted incompatible modes")
	}
}
func TestNegativeDesktopRightEdge(t *testing.T) {
	f := fixture()
	f.displays[0].Mode.X = -3000
	f.displays[0].Mode.Y = -200
	l, err := Acquire(context.Background(), f, 1280, 720)
	if err != nil {
		t.Fatal(err)
	}
	if l.Mode.X != -1080 || l.Mode.Y != -200 {
		t.Fatalf("wrong negative-coordinate placement: %+v", l.Mode)
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExactCaptureModeRequiresRequestedResolution(t *testing.T) {
	modes := []Mode{mode(1920, 1080), mode(1024, 768), mode(1280, 960), mode(1600, 1200), mode(2048, 1536)}
	for _, want := range modes[1:] {
		got, err := ExactCaptureMode(modes, want.Width, want.Height)
		if err != nil || got != want {
			t.Fatal(got, err)
		}
	}
	if _, err := ExactCaptureMode([]Mode{mode(1920, 1080), mode(800, 600)}, 1024, 768); err == nil {
		t.Fatal("silently substituted another resolution")
	}
	if _, err := ExactCaptureMode([]Mode{{Width: 1024, Height: 768, Refresh: 59, BitsPerPixel: 32}}, 1024, 768); err == nil {
		t.Fatal("unsupported refresh accepted")
	}
}

func TestAcquirePositionsOutsideDesktop(t *testing.T) {
	for _, tc := range []struct {
		position string
		x, y     int32
	}{
		{"right", 1920, -600}, {"left", -1124, -600}, {"above", -100, -1368}, {"below", 0, 1080},
	} {
		t.Run(tc.position, func(t *testing.T) {
			f := fixture()
			f.displays = append(f.displays, Display{Name: "upper", HardwareID: "physical2", Mode: Mode{X: -100, Y: -600, Width: 2020, Height: 600, Attached: true}})
			l, e := AcquirePosition(context.Background(), f, 1024, 768, tc.position)
			if e != nil {
				t.Fatal(e)
			}
			if l.Mode.X != tc.x || l.Mode.Y != tc.y {
				t.Fatalf("got %+v want %d,%d", l.Mode, tc.x, tc.y)
			}
			for _, d := range f.displays {
				if d.Name == "virtual" {
					continue
				}
				a, b := l.Mode, d.Mode
				if a.X < b.X+b.Width && a.X+a.Width > b.X && a.Y < b.Y+b.Height && a.Y+a.Height > b.Y {
					t.Fatal("overlapping display", d)
				}
			}
			if e = l.Close(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestAcquirePositionRejectsInvalidAndUnderflow(t *testing.T) {
	for _, position := range []string{"sideways", "left", "above"} {
		f := fixture()
		f.displays[0].Mode.X = -2147483648
		f.displays[0].Mode.Y = -2147483648
		if _, e := AcquirePosition(context.Background(), f, 1024, 768, position); e == nil {
			t.Fatal("accepted", position)
		}
		if len(f.sets) != 0 {
			t.Fatal("mutated invalid topology")
		}
	}
}

func TestDisconnectDetachesPreviouslyActiveVirtualDisplay(t *testing.T) {
	f := fixture()
	physical := f.displays[0]
	l, e := Acquire(context.Background(), f, 1024, 768)
	if e != nil {
		t.Fatal(e)
	}
	if e = l.CloseDetached(); e != nil {
		t.Fatal(e)
	}
	if f.displays[1].Mode.Attached || f.displays[0] != physical {
		t.Fatal("disconnected display remains active or physical changed")
	}
	if e = l.CloseDetached(); e != nil {
		t.Fatal(e)
	}
}

func TestDisconnectPreservesExternalDisplayChanges(t *testing.T) {
	f := fixture()
	l, e := Acquire(context.Background(), f, 1024, 768)
	if e != nil {
		t.Fatal(e)
	}
	f.displays[1].Mode.X = 42
	changed := f.displays[1]
	if e = l.CloseDetached(); e == nil {
		t.Fatal("expected external change protection")
	}
	if f.displays[1] != changed {
		t.Fatal("external display configuration overwritten")
	}
}
