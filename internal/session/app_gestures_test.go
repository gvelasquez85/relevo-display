package session

import (
	"context"
	"howett.net/plist"
	"net"
	"relevo/internal/wire"
	"strings"
	"testing"
	"time"
)

func TestApplicationGesturesEnableRemoteZoomBeforeVideo(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	commands := make(chan []string, 1)
	go func() {
		var got []string
		b, _ := plist.Marshal(map[string]interface{}{"protocolVersion": "1.0.20"}, plist.BinaryFormat)
		wire.Write(p, wire.Frame{Type: 100, Payload: b})
		for {
			f, e := wire.Read(p)
			if e != nil {
				return
			}
			if f.Type != 101 {
				continue
			}
			s, _ := wire.ParseText(f.Payload)
			got = append(got, s)
			if strings.HasPrefix(s, "size:") {
				b, _ = plist.Marshal(map[string]interface{}{"eventName": "ReadyToReceive"}, plist.BinaryFormat)
				wire.Write(p, wire.Frame{Type: 104, Payload: b})
			}
			if s == "startStream:0" {
				commands <- got
				p.Close()
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	Run(ctx, h, Options{FPS: 30, Duration: time.Second, AppGestures: true})
	select {
	case got := <-commands:
		init, mode, start := -1, -1, -1
		for i, s := range got {
			switch s {
			case "initZoomMode:1":
				init = i
			case "zoomMode:1":
				mode = i
			case "startStream:0":
				start = i
			}
		}
		if init < 0 || mode < init || start < mode {
			t.Fatal("remote zoom not negotiated before video", got)
		}
	default:
		t.Fatal("handshake did not finish")
	}
}
