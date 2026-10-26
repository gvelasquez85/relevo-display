package nativesession

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"net"
	"relevo/internal/nativewire"
	"sync/atomic"
	"testing"
	"time"
)

func jpegFrame(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if e := jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, 1024, 768)), nil); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func clientHello(t *testing.T, c net.Conn) {
	t.Helper()
	if e := nativewire.Write(c, nativewire.Frame{Type: nativewire.Hello, Payload: []byte(`{"protocol":1,"width":1024,"height":768}`)}); e != nil {
		t.Fatal(e)
	}
	f, e := nativewire.Read(c)
	if e != nil || f.Type != nativewire.Start {
		t.Fatal(f, e)
	}
	if e = nativewire.Write(c, nativewire.Frame{Type: nativewire.Status, Payload: []byte(`{"state":"ready"}`)}); e != nil {
		t.Fatal(e)
	}
}
func TestHandshakeAckBackpressureAndCancel(t *testing.T) {
	host, client := net.Pipe()
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := jpegFrame(t)
	var captures atomic.Int32
	done := make(chan error, 1)
	go func() {
		_, e := Run(ctx, host, Options{FPS: 60, Next: func(context.Context) ([]byte, error) { captures.Add(1); return b, nil }})
		done <- e
	}()
	clientHello(t, client)
	f, e := nativewire.Read(client)
	if e != nil || f.Type != nativewire.DisplayJPEG || f.Sequence != 1 {
		t.Fatal(f.Type, e)
	}
	time.Sleep(40 * time.Millisecond)
	if captures.Load() != 1 {
		t.Fatal("captured while awaiting ACK")
	}
	if e = nativewire.Write(client, nativewire.Frame{Type: nativewire.Ping, Sequence: 71}); e != nil {
		t.Fatal(e)
	}
	pong, e := nativewire.Read(client)
	if e != nil || pong.Type != nativewire.Pong || pong.Sequence != 71 {
		t.Fatal(pong, e)
	}
	nativewire.Write(client, nativewire.Frame{Type: nativewire.Touch, Payload: []byte(`{"phase":"down","x":0.2,"y":0.3}`)})
	nativewire.Write(client, nativewire.Frame{Type: nativewire.FrameApplied, Sequence: 1})
	f, e = nativewire.Read(client)
	if e != nil || f.Sequence != 2 {
		t.Fatal(f, e)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation stalled")
	}
}
func TestRejectWrongAckAndTouch(t *testing.T) {
	for _, bad := range []nativewire.Frame{{Type: nativewire.FrameApplied, Sequence: 99}, {Type: nativewire.Touch, Payload: []byte(`{"phase":"down","x":2,"y":0}`)}, {Type: nativewire.Touch, Payload: []byte(`{"phase":"down","y":0}`)}} {
		host, client := net.Pipe()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		b := jpegFrame(t)
		done := make(chan error, 1)
		go func() {
			_, e := Run(ctx, host, Options{FPS: 60, Next: func(context.Context) ([]byte, error) { return b, nil }})
			done <- e
		}()
		clientHello(t, client)
		if _, e := nativewire.Read(client); e != nil {
			t.Fatal(e)
		}
		nativewire.Write(client, bad)
		if e := <-done; e == nil {
			t.Fatal("invalid message accepted")
		}
		client.Close()
		cancel()
	}
}
func TestRejectWrongHelloAndCancelBeforeHello(t *testing.T) {
	host, client := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, e := Run(ctx, host, Options{FPS: 30, Next: func(context.Context) ([]byte, error) { t.Error("capture before handshake"); return nil, nil }})
		done <- e
	}()
	nativewire.Write(client, nativewire.Frame{Type: nativewire.Hello, Payload: []byte(`{"protocol":2,"width":1024,"height":768}`)})
	if e := <-done; e == nil {
		t.Fatal("wrong protocol accepted")
	}
	client.Close()
	host, client = net.Pipe()
	defer client.Close()
	go func() {
		_, e := Run(ctx, host, Options{FPS: 30, Next: func(context.Context) ([]byte, error) { return nil, nil }})
		done <- e
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel before hello stalled")
	}
}

func TestRequestedCapabilitiesMustBeAcknowledged(t *testing.T) {
	h, c := net.Pipe()
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, e := Run(ctx, h, Options{FPS: 30, Next: func(context.Context) ([]byte, error) { t.Error("captured before capability ack"); return nil, nil }, AudioEnabled: true})
		done <- e
	}()
	clientHello(t, c)
	if e := <-done; e == nil {
		t.Fatal("unacknowledged audio accepted")
	}
}
func TestAudioAndTouchWhileFrameOutstanding(t *testing.T) {
	h, c := net.Pipe()
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	audio := make(chan []byte, 1)
	touch := make(chan TouchEvent, 1)
	b := jpegFrame(t)
	go func() {
		_, e := Run(ctx, h, Options{FPS: 60, Next: func(context.Context) ([]byte, error) { return b, nil }, TouchEnabled: true, AudioEnabled: true, Audio: audio, OnTouch: func(e TouchEvent) error { touch <- e; return nil }})
		done <- e
	}()
	nativewire.Write(c, nativewire.Frame{Type: nativewire.Hello, Payload: []byte(`{"protocol":1,"width":1024,"height":768}`)})
	nativewire.Read(c)
	nativewire.Write(c, nativewire.Frame{Type: nativewire.Status, Payload: []byte(`{"state":"ready","touch":true,"audio":true}`)})
	f, e := nativewire.Read(c)
	if e != nil || f.Type != nativewire.DisplayJPEG {
		t.Fatal(f.Type, e)
	}
	nativewire.Write(c, nativewire.Frame{Type: nativewire.Touch, Payload: []byte(`{"phase":"down","x":0.5,"y":0.5}`)})
	select {
	case <-touch:
	case <-ctx.Done():
		t.Fatal("touch blocked by frame ACK")
	}
	pcm := make([]byte, 890)
	pcm[0] = 0x22
	pcm[1] = 0x56
	pcm[4] = 1
	pcm[6] = 16
	audio <- pcm
	f, e = nativewire.Read(c)
	if e != nil || f.Type != nativewire.AudioPCM {
		t.Fatal(f.Type, e)
	}
	cancel()
	<-done
}

func TestRejectInvalidCameraAndAudio(t *testing.T) {
	for _, camera := range []bool{false, true} {
		h, c := net.Pipe()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		b := jpegFrame(t)
		done := make(chan error, 1)
		go func() {
			_, e := Run(ctx, h, Options{FPS: 60, Next: func(context.Context) ([]byte, error) { return b, nil }, CameraEnabled: camera})
			done <- e
		}()
		nativewire.Write(c, nativewire.Frame{Type: nativewire.Hello, Payload: []byte(`{"protocol":1,"width":1024,"height":768}`)})
		nativewire.Read(c)
		nativewire.Write(c, nativewire.Frame{Type: nativewire.Status, Payload: []byte(`{"state":"ready","camera":true}`)})
		nativewire.Write(c, nativewire.Frame{Type: nativewire.CameraJPEG, Payload: []byte("not JPEG")})
		if e := <-done; e == nil {
			t.Fatal("invalid camera accepted")
		}
		cancel()
		c.Close()
	}
}

func TestAudioSourceEndIsReported(t *testing.T) {
	h, c := net.Pipe()
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	audio := make(chan []byte)
	close(audio)
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, h, Options{FPS: 1, Next: func(context.Context) ([]byte, error) { return nil, nil }, AudioEnabled: true, Audio: audio})
		done <- err
	}()
	nativewire.Write(c, nativewire.Frame{Type: nativewire.Hello, Payload: []byte(`{"protocol":1,"width":1024,"height":768}`)})
	nativewire.Read(c)
	nativewire.Write(c, nativewire.Frame{Type: nativewire.Status, Payload: []byte(`{"state":"ready","audio":true}`)})
	select {
	case err := <-done:
		if err == nil || err.Error() != "audio source ended" {
			t.Fatalf("unexpected audio termination: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("audio end silently ignored")
	}
}

func TestCameraDeliveryWhileDisplayOutstanding(t *testing.T) {
	h, c := net.Pipe()
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	display := jpegFrame(t)
	var camera bytes.Buffer
	if err := jpeg.Encode(&camera, image.NewGray(image.Rect(0, 0, 640, 480)), nil); err != nil {
		t.Fatal(err)
	}
	got := make(chan []byte, 1)
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, h, Options{FPS: 60, Next: func(context.Context) ([]byte, error) { return display, nil }, CameraEnabled: true, Camera: func(b []byte) error { got <- b; return nil }})
		done <- err
	}()
	nativewire.Write(c, nativewire.Frame{Type: nativewire.Hello, Payload: []byte(`{"protocol":1,"width":1024,"height":768}`)})
	nativewire.Read(c)
	nativewire.Write(c, nativewire.Frame{Type: nativewire.Status, Payload: []byte(`{"state":"ready","camera":true}`)})
	f, err := nativewire.Read(c)
	if err != nil || f.Type != nativewire.DisplayJPEG {
		t.Fatal(err, f.Type)
	}
	if err := nativewire.Write(c, nativewire.Frame{Type: nativewire.CameraJPEG, Payload: camera.Bytes()}); err != nil {
		t.Fatal(err)
	}
	select {
	case b := <-got:
		if !bytes.Equal(b, camera.Bytes()) {
			t.Fatal("camera payload changed")
		}
	case <-ctx.Done():
		t.Fatal("camera blocked by display ACK")
	}
	nativewire.Write(c, nativewire.Frame{Type: nativewire.FrameApplied, Sequence: f.Sequence})
	cancel()
	<-done
}
