package nativesession

import (
	"encoding/json"
	"fmt"
	"net"
	"relevo/internal/nativewire"
	"relevo/internal/remoteinput"
	"sync/atomic"
	"testing"
	"time"
)

func writeFocusContact(t *testing.T, p net.Conn, seq uint32, id int, phase string) {
	t.Helper()
	if e := nativewire.Write(p, nativewire.Frame{Type: nativewire.MultiTouch, Sequence: seq, Payload: []byte(fmt.Sprintf(`{"contacts":[{"id":%d,"phase":%q,"x":0.2,"y":0.3}]}`, id, phase))}); e != nil {
		t.Fatal(e)
	}
}
func focusTestOptions(probe func(float64, float64) bool) Options {
	return Options{MultiTouchEnabled: true, KeyboardEnabled: true, InputFocusEnabled: true, InputModesEnabled: true, OnContacts: func([]remoteinput.Contact) error { return nil }, OnKey: func(remoteinput.Key) error { return nil }, EditableAt: probe}
}

const focusReady = `{"state":"ready","multitouch":true,"keyboard":true,"inputFocus":true,"inputModes":true}`

func TestEditableFocusRetriesUntilWindowsProcessesTap(t *testing.T) {
	var probes atomic.Int32
	p, done, cancel := extensionSession(t, focusTestOptions(func(float64, float64) bool { return probes.Add(1) >= 3 }), focusReady)
	writeFocusContact(t, p, 71, 1, "down")
	writeFocusContact(t, p, 71, 1, "up")
	p.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	f, e := nativewire.Read(p)
	var reply struct {
		Editable bool   `json:"editable"`
		Sequence uint32 `json:"sequence"`
	}
	if e != nil || f.Type != nativewire.InputFocus || f.Sequence != 71 {
		t.Fatal(f, e)
	}
	if e = json.Unmarshal(f.Payload, &reply); e != nil || !reply.Editable || reply.Sequence != 71 || probes.Load() < 3 {
		t.Fatal("focus was not retried", string(f.Payload), probes.Load(), e)
	}
	cancel()
	<-done
}
func TestPendingFocusCancelledByNewContactOrMode(t *testing.T) {
	for _, mode := range []bool{false, true} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			var probes atomic.Int32
			p, done, cancel := extensionSession(t, focusTestOptions(func(float64, float64) bool { probes.Add(1); return false }), focusReady)
			writeFocusContact(t, p, 81, 1, "down")
			writeFocusContact(t, p, 81, 1, "up")
			if mode {
				if e := nativewire.Write(p, nativewire.Frame{Type: nativewire.InputMode, Payload: []byte(`{"mode":"keyboard"}`)}); e != nil {
					t.Fatal(e)
				}
			} else {
				writeFocusContact(t, p, 82, 1, "down")
				writeFocusContact(t, p, 83, 2, "down")
				writeFocusContact(t, p, 83, 1, "up")
				writeFocusContact(t, p, 83, 2, "up")
			}
			// A ping confirms the cancellation was processed without waiting for a focus reply.
			if e := nativewire.Write(p, nativewire.Frame{Type: nativewire.Ping, Sequence: 90}); e != nil {
				t.Fatal(e)
			}
			p.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			f, e := nativewire.Read(p)
			if e != nil || f.Type != nativewire.Pong {
				t.Fatal("unexpected stale focus", f, e)
			}
			p.SetReadDeadline(time.Now().Add(220 * time.Millisecond))
			f, e = nativewire.Read(p)
			if e == nil {
				t.Fatal("cancelled tap produced reply", f)
			}
			cancel()
			<-done
		})
	}
}

func TestFocusRetryDeadlineStillServesControlMessages(t *testing.T) {
	var probes atomic.Int32
	p, done, cancel := extensionSession(t, focusTestOptions(func(float64, float64) bool { probes.Add(1); return false }), focusReady)
	started := time.Now()
	writeFocusContact(t, p, 91, 1, "down")
	writeFocusContact(t, p, 91, 1, "up")
	if e := nativewire.Write(p, nativewire.Frame{Type: nativewire.Ping, Sequence: 92}); e != nil {
		t.Fatal(e)
	}
	p.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	f, e := nativewire.Read(p)
	if e != nil || f.Type != nativewire.Pong || f.Sequence != 92 {
		t.Fatal("focus retries blocked control pump", f, e)
	}
	p.SetReadDeadline(time.Now().Add(350 * time.Millisecond))
	f, e = nativewire.Read(p)
	var reply struct {
		Editable bool `json:"editable"`
	}
	if e != nil || f.Type != nativewire.InputFocus || f.Sequence != 91 || json.Unmarshal(f.Payload, &reply) != nil || reply.Editable {
		t.Fatal("missing bounded negative focus result", f, e)
	}
	if probes.Load() < 2 || time.Since(started) < 100*time.Millisecond {
		t.Fatal("negative result sent before retry window", probes.Load(), time.Since(started))
	}
	cancel()
	<-done
}
func TestNewTapSupersedesPendingFocusSequence(t *testing.T) {
	var probes atomic.Int32
	p, done, cancel := extensionSession(t, focusTestOptions(func(float64, float64) bool { return probes.Add(1) >= 2 }), focusReady)
	for _, seq := range []uint32{101, 102} {
		writeFocusContact(t, p, seq, 1, "down")
		writeFocusContact(t, p, seq, 1, "up")
	}
	p.SetReadDeadline(time.Now().Add(350 * time.Millisecond))
	f, e := nativewire.Read(p)
	if e != nil || f.Type != nativewire.InputFocus || f.Sequence != 102 {
		t.Fatal("stale tap opened keyboard", f, e)
	}
	p.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if f, e = nativewire.Read(p); e == nil {
		t.Fatal("superseded focus result leaked", f)
	}
	cancel()
	<-done
}
