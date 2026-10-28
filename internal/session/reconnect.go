package session

import (
	"errors"
	"io"
	"net"
)

// ShouldReconnect identifies receiver lifecycle/transport ends, never capture,
// decoder, malformed protocol or explicit host cancellation failures.
func ShouldReconnect(st Stats) bool {
	switch st.EndReason {
	case EndDeviceBackground, EndDeviceLocked, EndDeviceTerminate, EndPeerTimeout,
		EndTransportClosed, EndHandshakeTimeout, EndWriteFailure:
		return true
	}
	return false
}

// Called only at transport read/write boundaries. A capture source can itself
// return a network-shaped error; that does not make the display session retryable.
func connectionFailure(err error) bool {
	var op *net.OpError
	var timeout net.Error
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) || errors.As(err, &op) || (errors.As(err, &timeout) && timeout.Timeout())
}
