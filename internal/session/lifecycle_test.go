package session

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
	"relevo/internal/wire"
)

// compatiblePeer performs the actual compatible control handshake. The callback
// can stop consuming video to reproduce a connected but stalled receiver.
func compatiblePeer(p net.Conn, onFrame func(wire.Frame) bool) {
	defer p.Close()
	b, _ := plist.Marshal(map[string]interface{}{"protocolVersion": "1.0.20"}, plist.BinaryFormat)
	if wire.Write(p, wire.Frame{Type: 100, Payload: b}) != nil {
		return
	}
	for {
		f, e := wire.Read(p)
		if e != nil {
			return
		}
		if f.Type == 101 {
			s, _ := wire.ParseText(f.Payload)
			if strings.HasPrefix(s, "size:") {
				b, _ = plist.Marshal(map[string]interface{}{"eventName": "ReadyToReceive"}, plist.BinaryFormat)
				if wire.Write(p, wire.Frame{Type: 104, Payload: b}) != nil {
					return
				}
			}
		}
		if !onFrame(f) {
			return
		}
	}
}

func TestCancelInterruptsStalledVideoWrite(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	stalled := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	go compatiblePeer(p, func(f wire.Frame) bool {
		if f.Type == 101 {
			s, _ := wire.ParseText(f.Payload)
			if s == "startStream:0" {
				close(stalled)
				<-release
				return false
			}
		}
		return true
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, e := Run(ctx, h, Options{Continuous: true, FPS: 60}); done <- e }()
	select {
	case <-stalled:
	case <-time.After(time.Second):
		t.Fatal("handshake failed")
	}
	// Let the first video write block in the transport, then request GUI Stop.
	time.Sleep(80 * time.Millisecond)
	cancel()
	select {
	case e := <-done:
		if !errors.Is(e, context.Canceled) {
			t.Fatalf("cancellation lost: %v", e)
		}
	case <-time.After(350 * time.Millisecond):
		t.Fatal("Stop waits for the video write timeout")
	}
}

func TestPeerLifecycleEventsEndSession(t *testing.T) {
	for _, name := range []string{"DeviceTerminate", "DeviceLocked", "DeviceBackground"} {
		t.Run(name, func(t *testing.T) {
			h, p := net.Pipe()
			defer p.Close()
			go compatiblePeer(p, func(f wire.Frame) bool {
				if f.Type == 108 {
					b, _ := plist.Marshal(map[string]interface{}{"eventName": name}, plist.BinaryFormat)
					_ = wire.Write(p, wire.Frame{Type: 104, Payload: b})
					// Keep transport open to isolate the explicit lifecycle event.
					_, _ = io.Copy(io.Discard, p)
					return false
				}
				return true
			})
			st, e := Run(context.Background(), h, Options{Continuous: true, FPS: 30})
			want := map[string]string{"DeviceTerminate": EndDeviceTerminate, "DeviceLocked": EndDeviceLocked, "DeviceBackground": EndDeviceBackground}[name]
			if e == nil || st.EndReason != want {
				t.Fatalf("lifecycle event ignored: reason=%q err=%v", st.EndReason, e)
			}
		})
	}
}

func TestControlPongsKeepSessionAlive(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	go compatiblePeer(p, func(f wire.Frame) bool {
		if f.Type == 102 {
			return wire.Write(p, wire.Frame{Type: 103, Tag: f.Tag}) == nil
		}
		return true
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2400*time.Millisecond)
	defer cancel()
	st, e := Run(ctx, h, Options{Continuous: true, FPS: 2, PeerTimeout: 1800 * time.Millisecond})
	if !errors.Is(e, context.DeadlineExceeded) || st.Pongs < 2 || st.FramesSent < 3 {
		t.Fatalf("pongs did not maintain session: %+v %v", st, e)
	}
}

func TestClosedPeerEndsSession(t *testing.T) {
	h, p := net.Pipe()
	go compatiblePeer(p, func(f wire.Frame) bool { return f.Type != 108 })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, e := Run(ctx, h, Options{Continuous: true, FPS: 30})
	if e == nil || errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("close not detected: %v", e)
	}
}
