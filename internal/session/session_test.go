package session

import (
	"context"
	"fmt"
	"howett.net/plist"
	"net"
	"relevo/internal/wire"
	"strings"
	"testing"
	"time"
)

func TestHandshakePatternAndPong(t *testing.T) {
	host, peer := net.Pipe()
	defer peer.Close()
	got := make(chan bool, 1)
	go func() {
		b, _ := plist.Marshal(map[string]interface{}{"protocolVersion": "1.0.20"}, plist.BinaryFormat)
		wire.Write(peer, wire.Frame{Type: 100, Payload: b})
		ready := false
		for {
			f, e := wire.Read(peer)
			if e != nil {
				return
			}
			if f.Type == 101 {
				s, _ := wire.ParseText(f.Payload)
				if strings.HasPrefix(s, "size:") && !ready {
					ready = true
					b, _ = plist.Marshal(map[string]interface{}{"eventName": "ReadyToReceive"}, plist.BinaryFormat)
					wire.Write(peer, wire.Frame{Type: 104, Payload: b})
				}
			}
			if f.Type == 102 {
				wire.Write(peer, wire.Frame{Type: 103, Tag: f.Tag})
			}
			if f.Type == 108 {
				select {
				case got <- len(f.Payload) > 1000:
				default:
				}
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	st, e := Run(ctx, host, Options{Duration: time.Second, FPS: 2})
	if e != nil {
		t.Fatal(e)
	}
	if !st.PeerInfo || !st.Ready || st.FramesSent == 0 || st.ActualFPS <= 0 || st.ElapsedSeconds <= 0 || st.CaptureMillis <= 0 || st.WriteMillis <= 0 {
		t.Fatalf("%+v", st)
	}
	select {
	case ok := <-got:
		if !ok {
			t.Fatal("empty pattern")
		}
	default:
		t.Fatal("no video")
	}
}

func TestSilentPeerTimesOut(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	st, e := Run(ctx, h, Options{Duration: time.Second, FPS: 2})
	if e == nil || st.FramesSent != 0 {
		t.Fatal(st, e)
	}
}

func TestDecoderErrorIsFailure(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	go func() {
		b, _ := plist.Marshal(map[string]interface{}{"eventName": "ErrorDecoding"}, plist.BinaryFormat)
		wire.Write(p, wire.Frame{Type: 104, Payload: b})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	st, e := Run(ctx, h, Options{Duration: time.Second, FPS: 2})
	if e == nil || st.DecoderErrors != 1 || ShouldReconnect(st) {
		t.Fatal(st, e)
	}
}

func TestHandshakeHasIndependentDeadline(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	started := time.Now()
	st, e := Run(context.Background(), h, Options{Duration: 5 * time.Minute, FPS: 2, HandshakeTimeout: 30 * time.Millisecond})
	if e == nil || st.FramesSent != 0 || time.Since(started) > time.Second || st.EndReason != EndHandshakeTimeout || !ShouldReconnect(st) {
		t.Fatal(st, e, time.Since(started))
	}
}

func TestLateDecoderRejectionAfterStopIsFailure(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	go func() {
		b, _ := plist.Marshal(map[string]interface{}{"protocolVersion": "1.0.20"}, plist.BinaryFormat)
		wire.Write(p, wire.Frame{Type: 100, Payload: b})
		for {
			f, e := wire.Read(p)
			if e != nil {
				return
			}
			if f.Type == 101 {
				s, _ := wire.ParseText(f.Payload)
				if strings.HasPrefix(s, "size:") {
					b, _ = plist.Marshal(map[string]interface{}{"eventName": "ReadyToReceive"}, plist.BinaryFormat)
					wire.Write(p, wire.Frame{Type: 104, Payload: b})
				}
				if s == "stopStream" { /* first stop belongs to handshake */
				}
				if s == "startStream:0" {
					break
				}
			}
		}
		for {
			f, e := wire.Read(p)
			if e != nil {
				return
			}
			if f.Type == 102 {
				wire.Write(p, wire.Frame{Type: 103, Tag: f.Tag})
			}
			if f.Type == 101 {
				s, _ := wire.ParseText(f.Payload)
				if s == "stopStream" {
					b, _ = plist.Marshal(map[string]interface{}{"eventName": "ErrorDecoding"}, plist.BinaryFormat)
					wire.Write(p, wire.Frame{Type: 104, Payload: b})
					return
				}
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	st, e := Run(ctx, h, Options{Duration: 200 * time.Millisecond, FPS: 10})
	if e == nil || st.DecoderErrors != 1 {
		t.Fatal(st, e)
	}
}

func TestDesktopSourceFailureEndsSession(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	go func() {
		b, _ := plist.Marshal(map[string]interface{}{"protocolVersion": "1.0.20"}, plist.BinaryFormat)
		wire.Write(p, wire.Frame{Type: 100, Payload: b})
		for {
			f, e := wire.Read(p)
			if e != nil {
				return
			}
			if f.Type == 101 {
				s, _ := wire.ParseText(f.Payload)
				if strings.HasPrefix(s, "size:") {
					b, _ = plist.Marshal(map[string]interface{}{"eventName": "ReadyToReceive"}, plist.BinaryFormat)
					wire.Write(p, wire.Frame{Type: 104, Payload: b})
				}
			}
		}
	}()
	called := false
	_, e := Run(context.Background(), h, Options{Duration: time.Second, FPS: 20, Frame: func(ctx context.Context, index int) ([]byte, error) {
		called = true
		return nil, fmt.Errorf("capture unavailable")
	}})
	if !called || e == nil || !strings.Contains(e.Error(), "capture unavailable") {
		t.Fatal(called, e)
	}
}

func TestUnresponsiveReceiverEndsContinuousSession(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	go func() {
		b, _ := plist.Marshal(map[string]interface{}{"protocolVersion": "1.0.20"}, plist.BinaryFormat)
		wire.Write(p, wire.Frame{Type: 100, Payload: b})
		for {
			f, e := wire.Read(p)
			if e != nil {
				return
			}
			if f.Type == 101 {
				s, _ := wire.ParseText(f.Payload)
				if strings.HasPrefix(s, "size:") {
					b, _ = plist.Marshal(map[string]interface{}{"eventName": "ReadyToReceive"}, plist.BinaryFormat)
					wire.Write(p, wire.Frame{Type: 104, Payload: b})
				}
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, e := Run(ctx, h, Options{Continuous: true, FPS: 10, PeerTimeout: 80 * time.Millisecond})
	if e == nil || !strings.Contains(e.Error(), "not responding") {
		t.Fatalf("expected receiver timeout, got %v", e)
	}
}
