package wintray

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeSingleton struct {
	open        bool
	handles     int
	steps       int
	releaseAt   int
	windowValue singletonWindow
	commands    [][2]uintptr
	focused     uintptr
	leak        bool
	cancel      context.CancelFunc
	commandErr  error
}

func (f *fakeSingleton) create() (uintptr, bool, error) {
	f.handles++
	return uintptr(f.steps + 1), f.open, nil
}
func (f *fakeSingleton) close(uintptr)                    { f.handles-- }
func (f *fakeSingleton) window() (singletonWindow, error) { return f.windowValue, nil }
func (f *fakeSingleton) focus(h uintptr)                  { f.focused = h }
func (f *fakeSingleton) command(h, id uintptr) error {
	f.commands = append(f.commands, [2]uintptr{h, id})
	return f.commandErr
}
func (f *fakeSingleton) wait(ctx context.Context) error {
	if f.handles != 0 {
		f.leak = true
		return errors.New("duplicate mutex handle kept open")
	}
	f.steps++
	if f.windowValue.lab != 0 {
		f.windowValue.lab = 0
	} else if f.releaseAt > 0 && f.steps >= f.releaseAt {
		f.open = false
	}
	if f.cancel != nil {
		f.cancel()
	}
	return ctx.Err()
}
func TestShellVersionComparison(t *testing.T) {
	tests := []struct {
		old, new string
		older    bool
	}{
		{"Relevo Display 0.9", "Relevo Display 0.10.0", true},
		{"Relevo Display 0.12.3", "Relevo Display 0.12.5", true},
		{"Relevo Display 0.12.5", "Relevo Display 0.12.5", false},
		{"Relevo Display 0.13.0", "Relevo Display 0.12.5", false},
		{"Relevo Display 1.0.0", "Relevo Display 0.99.9", false},
	}
	for _, tt := range tests {
		a, e := parseShellVersion(tt.old)
		if e != nil {
			t.Fatal(e)
		}
		b, e := parseShellVersion(tt.new)
		if e != nil {
			t.Fatal(e)
		}
		if a.older(b) != tt.older {
			t.Fatalf("%s compared %s", tt.old, tt.new)
		}
	}
	for _, s := range []string{"Other Display 0.12.3", "Relevo Display unknown", "Relevo Display 0.-1.0", "Relevo Display 0.12.3 extra", "Relevo Display 0.12.x", "Relevo Display 1.2.3.4"} {
		if _, e := parseShellVersion(s); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
func TestSingletonOldActiveWaitsForOrdinaryCleanup(t *testing.T) {
	f := &fakeSingleton{open: true, releaseAt: 3, windowValue: singletonWindow{main: 10, title: "Relevo Display 0.12.3"}}
	h, existing, e := acquireShell(context.Background(), "Relevo Display 0.12.5", f)
	if e != nil || existing || h == 0 {
		t.Fatalf("%v %t %d", e, existing, h)
	}
	if f.steps != 3 || f.handles != 1 || f.leak {
		t.Fatalf("did not await cleanup safely: %+v", f)
	}
	for _, c := range f.commands {
		if c != [2]uintptr{10, 6} {
			t.Fatal("unexpected close command")
		}
	}
}
func TestSingletonClosesLabBeforeMain(t *testing.T) {
	f := &fakeSingleton{open: true, releaseAt: 2, windowValue: singletonWindow{main: 10, lab: 20, title: "Relevo Display 0.12.3"}}
	_, _, e := acquireShell(context.Background(), "Relevo Display 0.12.5", f)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(f.commands, [][2]uintptr{{20, 3}, {10, 6}}) {
		t.Fatalf("unsafe close sequence: %v", f.commands)
	}
	if f.leak {
		t.Fatal("leaked duplicate handle")
	}
}
func TestSingletonSameAndNewerOnlyFocus(t *testing.T) {
	for _, title := range []string{"Relevo Display 0.12.5", "Relevo Display 0.13.0"} {
		f := &fakeSingleton{open: true, windowValue: singletonWindow{main: 10, lab: 20, title: title}}
		h, existing, e := acquireShell(context.Background(), "Relevo Display 0.12.5", f)
		if e != nil || !existing || h != 0 || f.focused != 20 || len(f.commands) != 0 || f.handles != 0 {
			t.Fatalf("same/newer behavior: %+v %v", f, e)
		}
	}
}
func TestSingletonTimeoutNeverKeepsDuplicateHandle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fakeSingleton{open: true, cancel: cancel, windowValue: singletonWindow{main: 10, title: "Relevo Display 0.12.3"}}
	h, existing, e := acquireShell(ctx, "Relevo Display 0.12.5", f)
	if !errors.Is(e, context.Canceled) || existing || h != 0 || f.handles != 0 || !f.open || f.leak {
		t.Fatalf("timeout violated ownership: %+v %v", f, e)
	}
}
func TestSingletonUnknownVersionNeverCloses(t *testing.T) {
	f := &fakeSingleton{open: true, windowValue: singletonWindow{main: 10, title: "Other Application"}}
	_, _, e := acquireShell(context.Background(), "Relevo Display 0.12.5", f)
	if e == nil || len(f.commands) != 0 || f.handles != 0 {
		t.Fatal("unknown application was closed or duplicatehandle retained")
	}
}

func TestSingletonMissingWindowWaitsWithoutClosing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fakeSingleton{open: true, cancel: cancel}
	_, _, err := acquireShell(ctx, "Relevo Display 0.12.5", f)
	if !errors.Is(err, context.Canceled) || len(f.commands) != 0 || f.handles != 0 || f.leak {
		t.Fatalf("missing window unsafe: %+v %v", f, err)
	}
}

func TestSingletonCommandFailurePreservesExistingProcess(t *testing.T) {
	blocked := errors.New("PostMessage access denied")
	f := &fakeSingleton{open: true, commandErr: blocked, windowValue: singletonWindow{main: 10, title: "Relevo Display 0.12.3"}}
	h, existing, err := acquireShell(context.Background(), "Relevo Display 0.12.5", f)
	if !errors.Is(err, blocked) || h != 0 || existing || !f.open || f.handles != 0 || f.steps != 0 {
		t.Fatalf("command failure unsafe: %+v %v", f, err)
	}
}
