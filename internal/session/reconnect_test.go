package session

import (
	"context"
	"errors"
	"io"
	"net"
	"relevo/internal/wire"
	"testing"
	"time"
)

func TestReconnectClassificationExcludesPermanentFailures(t *testing.T) {
	for _, reason := range []string{EndCanceled, EndReadFailure, "", "decoder_error", "capture_source"} {
		if ShouldReconnect(Stats{EndReason: reason}) {
			t.Fatal("permanent failure retried", reason)
		}
	}
	for _, reason := range []string{EndDeviceBackground, EndDeviceLocked, EndDeviceTerminate, EndPeerTimeout, EndTransportClosed, EndHandshakeTimeout, EndWriteFailure} {
		if !ShouldReconnect(Stats{EndReason: reason}) {
			t.Fatal("receiver cannot resume", reason)
		}
	}
}

type shortWriteDeadline struct{ net.Conn }

func (c shortWriteDeadline) SetWriteDeadline(time.Time) error {
	return c.Conn.SetWriteDeadline(time.Now().Add(30 * time.Millisecond))
}
func TestVideoTransportWriteTimeoutAllowsReconnect(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	release := make(chan struct{})
	defer close(release)
	go compatiblePeer(p, func(f wire.Frame) bool {
		if f.Type == 101 {
			s, _ := wire.ParseText(f.Payload)
			if s == "startStream:0" {
				<-release
				return false
			}
		}
		return true
	})
	st, err := Run(context.Background(), shortWriteDeadline{h}, Options{Continuous: true, FPS: 30})
	if err == nil || st.EndReason != EndWriteFailure || !ShouldReconnect(st) {
		t.Fatal(st.EndReason, err)
	}
}
func TestCaptureNetworkShapedErrorDoesNotBecomeTransportRetry(t *testing.T) {
	for _, cause := range []error{errors.New("source failed"), net.ErrClosed, io.EOF, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			h, p := net.Pipe()
			defer p.Close()
			go compatiblePeer(p, func(wire.Frame) bool { return true })
			sourceErr := &net.OpError{Op: "capture", Net: "internal", Err: cause}
			st, err := Run(context.Background(), h, Options{Continuous: true, FPS: 30, Frame: func(context.Context, int) ([]byte, error) { return nil, sourceErr }})
			if !errors.Is(err, sourceErr) || ShouldReconnect(st) {
				t.Fatal(st.EndReason, err)
			}
		})
	}
}

func TestMalformedPeerInformationDoesNotRetry(t *testing.T) {
	h, p := net.Pipe()
	defer p.Close()
	go func() {
		_ = wire.Write(p, wire.Frame{Type: 100, Payload: []byte("not a plist")})
		_, _ = io.Copy(io.Discard, p)
	}()
	st, err := Run(context.Background(), h, Options{Continuous: true, FPS: 30})
	if err == nil || ShouldReconnect(st) {
		t.Fatal(st.EndReason, err)
	}
}
