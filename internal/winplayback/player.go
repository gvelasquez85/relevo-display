// Package winplayback provides explicit local microphone monitoring, not a
// Windows virtual microphone endpoint. It retains no recordings.
package winplayback

import (
	"fmt"
	"relevo/internal/nativewire"
	"sync"
)

type Backend interface {
	WritePCM([]byte) error
	Close() error
}
type Player struct {
	mu       sync.Mutex
	b        Backend
	closed   bool
	closeErr error
}

func New(b Backend) *Player { return &Player{b: b} }
func Open() (*Player, error) {
	b, e := nativeBackend()
	if e != nil {
		return nil, e
	}
	return New(b), nil
}
func (p *Player) Packet(b []byte) error {
	if e := nativewire.ValidatePCM(b); e != nil {
		return e
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.b == nil {
		return fmt.Errorf("audio monitor closed")
	}
	return p.b.WritePCM(b[8:])
}
func (p *Player) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		if p.b != nil {
			p.closeErr = p.b.Close()
		}
	}
	return p.closeErr
}
