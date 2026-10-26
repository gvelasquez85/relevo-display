package nativesession

import (
	"relevo/internal/nativewire"
	"relevo/internal/remoteinput"
	"testing"
	"time"
)

func TestKeyboardOnlyModePausesScreenFrames(t *testing.T) {
	p, done, cancel := extensionSession(t, Options{InputModesEnabled: true, KeyboardEnabled: true, OnKey: func(remoteinput.Key) error { return nil }}, `{"state":"ready","inputModes":true,"keyboard":true}`)
	if e := nativewire.Write(p, nativewire.Frame{Type: nativewire.InputMode, Payload: []byte(`{"mode":"keyboard"}`)}); e != nil {
		t.Fatal(e)
	}
	// Consume status through a ping/pong; no capture should occur after transition.
	if e := nativewire.Write(p, nativewire.Frame{Type: nativewire.Ping, Sequence: 17}); e != nil {
		t.Fatal(e)
	}
	f, e := nativewire.Read(p)
	if e != nil || f.Type != nativewire.Pong {
		t.Fatal(f, e)
	}
	p.SetReadDeadline(time.Now().Add(1100 * time.Millisecond))
	if f, e = nativewire.Read(p); e == nil {
		t.Fatal("screen transmitted in dedicated mode", f.Type)
	}
	cancel()
	<-done
}
func TestTextFocusReplyOnlyAfterTouchTap(t *testing.T) {
	p, done, cancel := extensionSession(t, Options{MultiTouchEnabled: true, KeyboardEnabled: true, InputFocusEnabled: true, OnContacts: func([]remoteinput.Contact) error { return nil }, OnKey: func(remoteinput.Key) error { return nil }, EditableAt: func(float64, float64) bool { return true }}, `{"state":"ready","multitouch":true,"keyboard":true,"inputFocus":true}`)
	for _, phase := range []string{"down", "up"} {
		if e := nativewire.Write(p, nativewire.Frame{Type: nativewire.MultiTouch, Sequence: 51, Payload: []byte(`{"contacts":[{"id":1,"phase":"` + phase + `","x":0.2,"y":0.3}]}`)}); e != nil {
			t.Fatal(e)
		}
	}
	f, e := nativewire.Read(p)
	if e != nil || f.Type != nativewire.InputFocus || f.Sequence != 51 {
		t.Fatal(f, e)
	}
	cancel()
	<-done
}

func TestDedicatedKeyboardStaysConnectedIdleThenResumesDisplay(t *testing.T) {
	p, done, cancel := extensionSession(t, Options{InputModesEnabled: true, KeyboardEnabled: true, OnKey: func(remoteinput.Key) error { return nil }}, `{"state":"ready","inputModes":true,"keyboard":true}`)
	nativewire.Write(p, nativewire.Frame{Type: nativewire.InputMode, Payload: []byte(`{"mode":"keyboard"}`)})
	for i := 0; i < 3; i++ {
		f, e := nativewire.Read(p)
		if e != nil || f.Type != nativewire.Ping {
			t.Fatal("idle keyboard disconnected or leaked display", f.Type, e)
		}
		if e = nativewire.Write(p, nativewire.Frame{Type: nativewire.Pong, Sequence: f.Sequence}); e != nil {
			t.Fatal(e)
		}
	}
	nativewire.Write(p, nativewire.Frame{Type: nativewire.InputMode, Payload: []byte(`{"mode":"screen"}`)})
	f, e := nativewire.Read(p)
	if e != nil || f.Type != nativewire.DisplayJPEG {
		t.Fatal("display did not resume", f.Type, e)
	}
	cancel()
	<-done
}

func TestLegacyTouchCanRequestEditableKeyboard(t *testing.T) {
	p, done, cancel := extensionSession(t, Options{TouchEnabled: true, KeyboardEnabled: true, InputFocusEnabled: true, OnTouch: func(TouchEvent) error { return nil }, OnKey: func(remoteinput.Key) error { return nil }, EditableAt: func(float64, float64) bool { return true }}, `{"state":"ready","touch":true,"keyboard":true,"inputFocus":true}`)
	for _, phase := range []string{"down", "up"} {
		if e := nativewire.Write(p, nativewire.Frame{Type: nativewire.Touch, Sequence: 9, Payload: []byte(`{"phase":"` + phase + `","x":0.2,"y":0.3}`)}); e != nil {
			t.Fatal(e)
		}
	}
	f, e := nativewire.Read(p)
	if e != nil || f.Type != nativewire.InputFocus || f.Sequence != 9 {
		t.Fatal(f, e)
	}
	cancel()
	<-done
}
func TestPinchDoesNotRequestKeyboard(t *testing.T) {
	var taps tapTracker
	taps.apply([]remoteinput.Contact{{ID: 1, Phase: "down", X: .2, Y: .2}, {ID: 2, Phase: "down", X: .7, Y: .7}})
	taps.apply([]remoteinput.Contact{{ID: 1, Phase: "up", X: .2, Y: .2}})
	if _, _, ok := taps.apply([]remoteinput.Contact{{ID: 2, Phase: "up", X: .7, Y: .7}}); ok {
		t.Fatal("pinch opened keyboard")
	}
}
