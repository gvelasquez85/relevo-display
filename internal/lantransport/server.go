// Package lantransport owns an ephemeral, explicitly paired LAN session.
// PairingURI contains credentials and must never be persisted or logged.
package lantransport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"net"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"
)

const Port = 52743

var ErrUnavailable = errors.New("LAN session unavailable")

// Server serves one authenticated session. Close closes every connection it owns.
type Server struct {
	mu              sync.Mutex
	listener        net.Listener
	config          *tls.Config
	token           [16]byte
	pin             string
	timeout         time.Duration
	pending         net.Conn
	accepted        net.Conn
	started, closed bool
}

// Addresses lists active private IPv4 addresses suitable for explicit LAN pairing.
func Addresses() []string {
	interfaces, e := net.Interfaces()
	if e != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, e := iface.Addrs()
		if e != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, e := net.ParseCIDR(addr.String())
			if e == nil && ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
				seen[ip.String()] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// New binds only a selected active private IPv4 interface, never all interfaces.
func New(host string) (*Server, error) {
	found := false
	for _, address := range Addresses() {
		if address == host {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("select an active local private IPv4 address")
	}
	return newServer(net.JoinHostPort(host, strconv.Itoa(Port)), 10*time.Second)
}
func newServer(address string, timeout time.Duration) (*Server, error) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		return nil, ErrUnavailable
	}
	serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		return nil, ErrUnavailable
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Relevo ephemeral pairing"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		return nil, ErrUnavailable
	}
	s := &Server{timeout: timeout}
	if _, e = rand.Read(s.token[:]); e != nil {
		return nil, ErrUnavailable
	}
	h := sha256.Sum256(der)
	s.pin = hex.EncodeToString(h[:])
	s.config = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384}}
	s.listener, e = net.Listen("tcp4", address)
	if e != nil {
		return nil, errors.New("could not open selected LAN interface")
	}
	return s, nil
}
func (s *Server) Addr() net.Addr { return s.listener.Addr() }
func (s *Server) PairingURI() string {
	host, port, _ := net.SplitHostPort(s.listener.Addr().String())
	q := url.Values{"host": {host}, "port": {port}, "pin": {s.pin}, "token": {hex.EncodeToString(s.token[:])}}
	return (&url.URL{Scheme: "relevo", Host: "pair", RawQuery: q.Encode()}).String()
}

// Accept rejects unauthenticated peers without handing their bytes to the app.
// It may be called only once; generate a new pairing for another session.
func (s *Server) Accept(ctx context.Context) (net.Conn, error) {
	s.mu.Lock()
	if s.started || s.closed {
		s.mu.Unlock()
		return nil, ErrUnavailable
	}
	s.started = true
	s.mu.Unlock()
	stop := context.AfterFunc(ctx, func() { s.Close() })
	defer stop()
	deadline := time.Now().Add(60 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if tcp, ok := s.listener.(*net.TCPListener); ok {
		tcp.SetDeadline(deadline)
	}
	for attempt := 0; attempt < 16; attempt++ {
		raw, e := s.listener.Accept()
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.Close()
			return nil, ErrUnavailable
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			raw.Close()
			return nil, ErrUnavailable
		}
		s.pending = raw
		s.mu.Unlock()
		perClient := time.Now().Add(s.timeout)
		if deadline.Before(perClient) {
			perClient = deadline
		}
		raw.SetDeadline(perClient)
		c := tls.Server(raw, s.config)
		e = c.HandshakeContext(ctx)
		var auth [21]byte
		if e == nil {
			_, e = io.ReadFull(c, auth[:])
		}
		if e == nil && (string(auth[:5]) != "RVLP1" || subtle.ConstantTimeCompare(auth[5:], s.token[:]) != 1) {
			e = ErrUnavailable
		}
		if e == nil {
			var n int
			n, e = c.Write([]byte("OK1"))
			if n != 3 && e == nil {
				e = io.ErrShortWrite
			}
		}
		s.mu.Lock()
		s.pending = nil
		if e == nil && !s.closed && ctx.Err() == nil {
			s.accepted = c
			c.SetDeadline(time.Time{})
			s.mu.Unlock()
			s.listener.Close()
			return c, nil
		}
		s.mu.Unlock()
		c.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	s.Close()
	return nil, ErrUnavailable
}
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	pending, accepted := s.pending, s.accepted
	s.mu.Unlock()
	s.listener.Close()
	if pending != nil {
		pending.Close()
	}
	if accepted != nil {
		accepted.Close()
	}
	return nil
}
