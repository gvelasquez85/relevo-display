package nativeapp

import (
	"context"
	"encoding/binary"
	"howett.net/plist"
	"io"
	"net"
	"relevo/internal/nativewire"
	"testing"
	"time"
)

func TestGUIRunnerStopsAndCanRunAgain(t *testing.T) {
	for trial := 0; trial < 2; trial++ {
		listener, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		received := make(chan struct{})
		serverDone := make(chan error, 1)
		go func() {
			for stage := 0; stage < 2; stage++ {
				c, e := listener.Accept()
				if e != nil {
					serverDone <- e
					return
				}
				c.SetDeadline(time.Now().Add(3 * time.Second))
				var h [16]byte
				if _, e = io.ReadFull(c, h[:]); e != nil {
					c.Close()
					serverDone <- e
					return
				}
				n := binary.LittleEndian.Uint32(h[:])
				if n < 16 || n > 65536 {
					c.Close()
					serverDone <- io.ErrUnexpectedEOF
					return
				}
				payload := make([]byte, n-16)
				if _, e = io.ReadFull(c, payload); e != nil {
					c.Close()
					serverDone <- e
					return
				}
				var response any = map[string]any{"Number": uint64(0)}
				if stage == 0 {
					response = map[string]any{"DeviceList": []any{map[string]any{"DeviceID": uint64(9), "Properties": map[string]any{"ConnectionType": "USB"}}}}
				}
				payload, e = plist.Marshal(response, plist.XMLFormat)
				if e != nil {
					c.Close()
					serverDone <- e
					return
				}
				binary.LittleEndian.PutUint32(h[:], uint32(16+len(payload)))
				if _, e = c.Write(append(h[:], payload...)); e != nil {
					c.Close()
					serverDone <- e
					return
				}
				if stage == 0 {
					c.Close()
					continue
				}
				if e = nativewire.Write(c, nativewire.Frame{Type: nativewire.Hello, Payload: []byte(`{"protocol":1,"width":1024,"height":768}`)}); e != nil {
					c.Close()
					serverDone <- e
					return
				}
				if _, e = nativewire.Read(c); e != nil {
					c.Close()
					serverDone <- e
					return
				}
				if e = nativewire.Write(c, nativewire.Frame{Type: nativewire.Status, Payload: []byte(`{"state":"ready"}`)}); e != nil {
					c.Close()
					serverDone <- e
					return
				}
				f, e := nativewire.Read(c)
				if e != nil || f.Type != nativewire.DisplayJPEG {
					c.Close()
					serverDone <- io.ErrUnexpectedEOF
					return
				}
				close(received)
				_, e = nativewire.Read(c)
				c.Close()
				if e != io.EOF {
					serverDone <- e
					return
				}
			}
			serverDone <- nil
		}()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- runTrial(ctx, trialConfig{Mode: "pattern", FPS: 30, Quality: 70, Duration: 5 * time.Minute, Endpoint: "tcp://" + listener.Addr().String()}, func(string) {})
		}()
		select {
		case <-received:
		case <-time.After(4 * time.Second):
			cancel()
			listener.Close()
			t.Fatal("test session did not start")
		}
		cancel()
		select {
		case e := <-done:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(time.Second):
			t.Fatal("Stop blocked")
		}
		if e := <-serverDone; e != nil {
			t.Fatal(e)
		}
		listener.Close()
	}
}
