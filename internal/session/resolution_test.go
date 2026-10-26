package session

import (
	"bytes"
	"context"
	"howett.net/plist"
	"image"
	"image/jpeg"
	"net"
	"relevo/internal/wire"
	"strings"
	"testing"
	"time"
)

func TestNegotiatesIPadDimensions(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	seen := make(chan string, 1)
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
					seen <- s
					b, _ = plist.Marshal(map[string]interface{}{"eventName": "ReadyToReceive"}, plist.BinaryFormat)
					wire.Write(p, wire.Frame{Type: 104, Payload: b})
				}
			}
		}
	}()
	var b bytes.Buffer
	jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1024, 768)), nil)
	_, e := Run(context.Background(), h, Options{Duration: 100 * time.Millisecond, FPS: 60, Width: 1024, Height: 768, Frame: func(context.Context, int) ([]byte, error) { return b.Bytes(), nil }})
	if e != nil {
		t.Fatal(e)
	}
	if got := <-seen; got != "size:1024x768" {
		t.Fatal(got)
	}
}
func TestRejectsInconsistentDimensions(t *testing.T) {
	for _, sz := range [][2]int{{1024, 0}, {0, 768}, {-1, 768}, {20000, 768}} {
		h, p := net.Pipe()
		_, e := Run(context.Background(), h, Options{Duration: time.Second, FPS: 5, Width: sz[0], Height: sz[1]})
		p.Close()
		if e == nil {
			t.Fatalf("accepted %v", sz)
		}
	}
}
func TestContinuousCanCancelWithoutHandshake(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := Run(ctx, h, Options{Continuous: true, FPS: 5})
	if e == nil || !strings.Contains(e.Error(), "canceled") {
		t.Fatal(e)
	}
}

func TestContinuousIgnoresBoundedDuration(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := time.Now()
	frames := 0
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
			if f.Type == 108 {
				frames++
				if frames == 4 {
					cancel()
				}
			}
		}
	}()
	st, e := Run(ctx, h, Options{Continuous: true, Duration: time.Millisecond, FPS: 20})
	if e == nil || !strings.Contains(e.Error(), "canceled") || st.FramesSent < 3 || time.Since(started) < 100*time.Millisecond {
		t.Fatal(st, e)
	}
}

func TestRejectsRatesOutsideSixtyFPSRange(t *testing.T) {
	for _, fps := range []int{-1, 0, 61, 120} {
		h, p := net.Pipe()
		_, e := Run(context.Background(), h, Options{Duration: time.Second, FPS: fps})
		p.Close()
		if e == nil || !strings.Contains(e.Error(), "FPS") {
			t.Fatalf("FPS%d: %v", fps, e)
		}
	}
}
