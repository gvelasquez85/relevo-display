package session

import (
	"context"
	"errors"
	"fmt"
	"howett.net/plist"
	"net"
	"reflect"
	"relevo/internal/wire"
	"strings"
	"testing"
	"time"
)

func TestInputDiagnosticsCountsKnownNamesWithoutInjection(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	go func() {
		for _, name := range []string{"SingleTap", "SingleTap", "TwoFingerSingleTap", "SingleTap2nd", "Scrolled", "Dragged", "HoverEvent", "KeyboardEvent"} {
			b, _ := plist.Marshal(map[string]interface{}{"eventName": name, "x": 12.5, "y": 99.5, "privateText": "never log"}, plist.BinaryFormat)
			if wire.Write(p, wire.Frame{Type: 104, Payload: b}) != nil {
				return
			}
		}
		b, _ := plist.Marshal(map[string]interface{}{"eventName": "DeviceLocked"}, plist.BinaryFormat)
		wire.Write(p, wire.Frame{Type: 104, Payload: b})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var logs []string
	st, e := Run(ctx, h, Options{Duration: time.Second, FPS: 5, Log: func(s string) { logs = append(logs, s) }})
	if e == nil || st.InputEventCounts["SingleTap"] != 2 || len(st.InputEventCounts) != 7 {
		t.Fatal(st, e)
	}
	if st.InputEventSchemas["SingleTap"]["x"] != "number" || st.InputEventSchemas["SingleTap"]["privateText"] != "" {
		t.Fatal("unsafe or missing schema", st.InputEventSchemas)
	}
	for _, line := range logs {
		if strings.Contains(line, "never log") {
			t.Fatal("input payload logged")
		}
	}
}

func inputCallbackPeer(p net.Conn, events []map[string]interface{}) {
	sent := false
	compatiblePeer(p, func(f wire.Frame) bool {
		if f.Type != 101 {
			return true
		}
		text, _ := wire.ParseText(f.Payload)
		if text != "startStream:0" || sent {
			return true
		}
		sent = true
		for _, event := range events {
			b, _ := plist.Marshal(event, plist.BinaryFormat)
			if wire.Write(p, wire.Frame{Type: 104, Payload: b}) != nil {
				return false
			}
		}
		b, _ := plist.Marshal(map[string]interface{}{"eventName": "DeviceLocked"}, plist.BinaryFormat)
		wire.Write(p, wire.Frame{Type: 104, Payload: b})
		return false
	})
}
func TestInputCallbackReceivesValidatedWireEventsOnly(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	events := []map[string]interface{}{
		{"eventName": "SingleTap", "x": 12.5, "y": 99.5},
		{"eventName": "KeyboardEvent", "text": "private typed words"},
		{"eventName": "UnknownPrivateEvent", "text": "unknown private text"},
		{"eventName": "SingleTap", "x": "bad coordinate"},
		{"eventName": "Scrolled", "deltaX": true},
		{"eventName": "Dragged", "mouseButton": true},
		{"eventName": "KeyboardPressEvent", "modifierFlags": "malformed"},
	}
	done := make(chan struct{})
	go func() { defer close(done); inputCallbackPeer(p, events) }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var got, logs []string
	st, err := Run(ctx, h, Options{Continuous: true, FPS: 30, Log: func(s string) { logs = append(logs, s) }, InputEvent: func(event map[string]interface{}) error { got = append(got, event["eventName"].(string)); return nil }})
	if err == nil || !reflect.DeepEqual(got, []string{"SingleTap", "KeyboardEvent"}) || st.InputEventCounts["SingleTap"] != 1 || st.InputEventCounts["KeyboardEvent"] != 1 || len(st.InputEventCounts) != 2 {
		t.Fatalf("unexpected dispatch/counts: %v %+v %v", got, st.InputEventCounts, err)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "remote input enabled") || strings.Contains(joined, "private typed") || strings.Contains(joined, "unknown private") || strings.Contains(joined, "99.5") {
		t.Fatal("unsafe or misleading input diagnostic")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("peer did not finish")
	}
}
func TestInputCallbackErrorIsOpaqueAndTerminatesWireSession(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		inputCallbackPeer(p, []map[string]interface{}{{"eventName": "KeyboardEvent", "text": "private typed text"}})
	}()
	failure := errors.New("private typed text failed at 12.5,99.5")
	var logs []string
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	st, err := Run(ctx, h, Options{Continuous: true, FPS: 30, Log: func(s string) { logs = append(logs, s) }, InputEvent: func(map[string]interface{}) error { return failure }})
	if !errors.Is(err, failure) || st.InputEventCounts["KeyboardEvent"] != 1 {
		t.Fatalf("callback error lost: %v", err)
	}
	diagnostic := fmt.Sprint(err) + strings.Join(logs, "\n")
	if strings.Contains(diagnostic, "private typed") || strings.Contains(diagnostic, "12.5") || strings.Contains(diagnostic, "99.5") {
		t.Fatal("callback leaked event data")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("session did not close peer")
	}
}
func TestInputCallbackCancellationStillClosesTransport(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		inputCallbackPeer(p, []map[string]interface{}{{"eventName": "SingleTap", "x": 1.0, "y": 2.0}})
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := Run(ctx, h, Options{Continuous: true, FPS: 30, InputEvent: func(map[string]interface{}) error { cancel(); return nil }})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("callback cancellation lost: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel did not close transport")
	}
}

func TestMalformedInputReleasesStateWithoutInterruptingSession(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	go inputCallbackPeer(p, []map[string]interface{}{
		{"eventName": "Dragged", "x": "invalid", "y": 0.5},
		{"eventName": "KeyboardEvent", "inputString": "private typed text"},
	})
	released, handled := 0, 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := Run(ctx, h, Options{Continuous: true, FPS: 30, InputCancel: func() { released++ }, InputEvent: func(map[string]interface{}) error { handled++; cancel(); return nil }})
	if !errors.Is(err, context.Canceled) || released != 1 || handled != 1 {
		t.Fatalf("cancel=%d input=%d err=%v", released, handled, err)
	}
}
