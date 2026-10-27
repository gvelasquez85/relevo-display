package lantransport

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"io"
	"net"
	"net/url"
	"testing"
	"time"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	s, e := newServer("127.0.0.1:0", 250*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func credentials(t *testing.T, s *Server) (string, []byte) {
	t.Helper()
	u, e := url.Parse(s.PairingURI())
	if e != nil {
		t.Fatal(e)
	}
	token, e := hex.DecodeString(u.Query().Get("token"))
	if e != nil || len(token) != 16 {
		t.Fatal("invalid token")
	}
	return u.Query().Get("pin"), token
}
func dial(t *testing.T, s *Server) *tls.Conn {
	t.Helper()
	c, e := tls.Dial("tcp", s.Addr().String(), &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c
}
func TestAuthenticatedStreamAndPin(t *testing.T) {
	s := testServer(t)
	pin, token := credentials(t, s)
	out := make(chan net.Conn, 1)
	go func() {
		c, e := s.Accept(context.Background())
		if e == nil {
			out <- c
		} else {
			out <- nil
		}
	}()
	c := dial(t, s)
	h := sha256.Sum256(c.ConnectionState().PeerCertificates[0].Raw)
	if hex.EncodeToString(h[:]) != pin {
		t.Fatal("pin mismatch")
	}
	if _, e := c.Write(append(append([]byte("RVLP1"), token...), []byte("RVL1")...)); e != nil {
		t.Fatal(e)
	}
	ack := make([]byte, 3)
	if _, e := io.ReadFull(c, ack); e != nil || string(ack) != "OK1" {
		t.Fatal("missing ack", e)
	}
	accepted := <-out
	if accepted == nil {
		t.Fatal("accept failed")
	}
	b := make([]byte, 4)
	if _, e := io.ReadFull(accepted, b); e != nil || string(b) != "RVL1" {
		t.Fatal("application framing consumed", e)
	}
	if _, e := s.Accept(context.Background()); e == nil {
		t.Fatal("second session accepted")
	}
	s.Close()
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, e := c.Read(b); e == nil {
		t.Fatal("close left session open")
	}
}
func TestWrongTokenRejectedThenValidClientAccepted(t *testing.T) {
	s := testServer(t)
	_, token := credentials(t, s)
	done := make(chan error, 1)
	go func() { _, e := s.Accept(context.Background()); done <- e }()
	bad := dial(t, s)
	bad.Write(append([]byte("RVLP1"), make([]byte, 16)...))
	bad.SetReadDeadline(time.Now().Add(time.Second))
	if _, e := bad.Read(make([]byte, 3)); e == nil {
		t.Fatal("bad token accepted")
	}
	good := dial(t, s)
	good.Write(append([]byte("RVLP1"), token...))
	if _, e := io.ReadFull(good, make([]byte, 3)); e != nil {
		t.Fatal(e)
	}
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}
func TestCancellationClosesPendingAuthentication(t *testing.T) {
	s := testServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, e := s.Accept(ctx); done <- e }()
	c := dial(t, s)
	cancel()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("missing cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel blocked")
	}
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, e := c.Read(make([]byte, 1)); e == nil {
		t.Fatal("pending connection open")
	}
}
func TestStalledClientDeadlineAllowsNextClient(t *testing.T) {
	s := testServer(t)
	_, token := credentials(t, s)
	done := make(chan error, 1)
	go func() { _, e := s.Accept(context.Background()); done <- e }()
	stalled := dial(t, s)
	stalled.SetReadDeadline(time.Now().Add(time.Second))
	if _, e := stalled.Read(make([]byte, 1)); e == nil {
		t.Fatal("stalled client survived")
	}
	good := dial(t, s)
	good.Write(append([]byte("RVLP1"), token...))
	if _, e := io.ReadFull(good, make([]byte, 3)); e != nil {
		t.Fatal(e)
	}
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}
func TestRejectTLS11(t *testing.T) {
	s := testServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Accept(ctx)
	c, e := tls.Dial("tcp", s.Addr().String(), &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11})
	if c != nil {
		c.Close()
	}
	if e == nil {
		t.Fatal("TLS1.1 accepted")
	}
}
func TestNewRejectsNonLocalAndPublicHosts(t *testing.T) {
	for _, host := range []string{"", "0.0.0.0", "127.0.0.1", "8.8.8.8", "192.0.2.15", "localhost", "10.254.253.252"} {
		if s, e := New(host); e == nil {
			s.Close()
			t.Fatalf("accepted %s", host)
		}
	}
}

func TestInvalidClientsHaveBoundedAttempts(t *testing.T) {
	s := testServer(t)
	done := make(chan error, 1)
	go func() { _, e := s.Accept(context.Background()); done <- e }()
	for i := 0; i < 16; i++ {
		c := dial(t, s)
		c.Write(append([]byte("WRONG"), make([]byte, 16)...))
		c.SetReadDeadline(time.Now().Add(time.Second))
		if _, e := c.Read(make([]byte, 3)); e == nil {
			t.Fatal("invalid header accepted")
		}
		c.Close()
	}
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("missing exhausted error")
		}
	case <-time.After(time.Second):
		t.Fatal("unbounded rejection")
	}
}
func TestCloseInterruptsIdleAccept(t *testing.T) {
	s := testServer(t)
	done := make(chan error, 1)
	go func() { _, e := s.Accept(context.Background()); done <- e }()
	s.Close()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("missing close error")
		}
	case <-time.After(time.Second):
		t.Fatal("idle accept not closed")
	}
}
