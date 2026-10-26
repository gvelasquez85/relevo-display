package nativesession

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"relevo/internal/nativewire"
	"relevo/internal/remoteinput"
	"strings"
	"testing"
	"time"
)

func extensionSession(t *testing.T, o Options, ready string) (net.Conn, <-chan error, context.CancelFunc) {
	t.Helper()
	host, peer := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	b := jpegFrame(t)
	o.FPS = 1
	o.Next = func(context.Context) ([]byte, error) { return b, nil }
	done := make(chan error, 1)
	go func() { _, e := Run(ctx, host, o); done <- e }()
	t.Cleanup(func() { cancel(); peer.Close() })
	if e := nativewire.Write(peer, nativewire.Frame{Type: nativewire.Hello, Payload: []byte(`{"protocol":1,"width":1024,"height":768}`)}); e != nil {
		t.Fatal(e)
	}
	start, e := nativewire.Read(peer)
	if e != nil {
		t.Fatal(e)
	}
	var caps map[string]any
	if e = json.Unmarshal(start.Payload, &caps); e != nil {
		t.Fatal(e)
	}
	if o.MicrophoneEnabled && caps["microphone"] != true || o.KeyboardEnabled && caps["keyboard"] != true || o.MultiTouchEnabled && caps["multitouch"] != true {
		t.Fatal("missing requested capabilities", caps)
	}
	if e = nativewire.Write(peer, nativewire.Frame{Type: nativewire.Status, Payload: []byte(ready)}); e != nil {
		t.Fatal(e)
	}
	return peer, done, cancel
}
func TestExtendedInputAndMicrophoneDelivery(t *testing.T) {
	touches := make(chan int, 1)
	keys := make(chan string, 1)
	mic := make(chan int, 1)
	p, done, cancel := extensionSession(t, Options{MultiTouchEnabled: true, KeyboardEnabled: true, MicrophoneEnabled: true,
		OnContacts: func(c []remoteinput.Contact) error { touches <- len(c); return nil },
		OnKey:      func(k remoteinput.Key) error { keys <- k.Text; return nil },
		Microphone: func(b []byte) error { mic <- len(b); return nil },
	}, `{"state":"ready","multitouch":true,"keyboard":true,"microphone":true}`)
	frame, e := nativewire.Read(p)
	if e != nil || frame.Type != nativewire.DisplayJPEG {
		t.Fatal("expected outstanding display frame", e)
	}
	// Deliberately do not ACK: input and microphone must flow while video waits.
	for _, f := range []nativewire.Frame{
		{Type: nativewire.MultiTouch, Payload: []byte(`{"contacts":[{"id":1,"phase":"down","x":0.2,"y":0.3},{"id":2,"phase":"down","x":0.7,"y":0.6}]}`)},
		{Type: nativewire.Keyboard, Payload: []byte(`{"kind":"text","text":"ñ á 😀"}`)},
	} {
		if e := nativewire.Write(p, f); e != nil {
			t.Fatal(e)
		}
	}
	b := make([]byte, 890)
	binary.LittleEndian.PutUint32(b, 22050)
	binary.LittleEndian.PutUint16(b[4:], 1)
	binary.LittleEndian.PutUint16(b[6:], 16)
	if e := nativewire.Write(p, nativewire.Frame{Type: nativewire.MicrophonePCM, Payload: b}); e != nil {
		t.Fatal(e)
	}
	if n := <-touches; n != 2 {
		t.Fatal(n)
	}
	if k := <-keys; k != "ñ á 😀" {
		t.Fatal(k)
	}
	if n := <-mic; n != 890 {
		t.Fatal(n)
	}
	cancel()
	<-done
}
func TestUnnegotiatedExtendedInputRejected(t *testing.T) {
	for _, f := range []nativewire.Frame{{Type: nativewire.MultiTouch, Payload: []byte(`{"contacts":[]}`)}, {Type: nativewire.Keyboard, Payload: []byte(`{"kind":"text","text":"a"}`)}, {Type: nativewire.MicrophonePCM, Payload: []byte{1}}} {
		p, done, _ := extensionSession(t, Options{}, `{"state":"ready"}`)
		if e := nativewire.Write(p, f); e != nil {
			t.Fatal(e)
		}
		if e := <-done; e == nil || !strings.Contains(e.Error(), "not negotiated") {
			t.Fatalf("expected capability refusal: %v", e)
		}
	}
}
func TestMissingExtensionAcknowledgmentRejected(t *testing.T) {
	_, done, _ := extensionSession(t, Options{MicrophoneEnabled: true, Microphone: func([]byte) error { return nil }}, `{"state":"ready"}`)
	if e := <-done; e == nil || !strings.Contains(e.Error(), "not acknowledged") {
		t.Fatal(e)
	}
}
func TestMissingTouchCoordinatesRejectedBeforeCallback(t *testing.T) {
	called := false
	p, done, _ := extensionSession(t, Options{MultiTouchEnabled: true, OnContacts: func([]remoteinput.Contact) error { called = true; return nil }}, `{"state":"ready","multitouch":true}`)
	nativewire.Write(p, nativewire.Frame{Type: nativewire.MultiTouch, Payload: []byte(`{"contacts":[{"id":1,"phase":"down","x":0.1}]}`)})
	if e := <-done; e == nil || called {
		t.Fatal("missing Y accepted", e, called)
	}
}
