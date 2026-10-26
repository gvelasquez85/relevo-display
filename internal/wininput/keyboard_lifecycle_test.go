package wininput

import (
	"errors"
	"relevo/internal/remoteinput"
	"sync/atomic"
	"testing"
	"time"
)

type keyboardLifecycleBackend struct {
	blocked  atomic.Bool
	attempts atomic.Int32
	released chan struct{}
}

func (*keyboardLifecycleBackend) Touch([]TouchPacket) error { return nil }
func (b *keyboardLifecycleBackend) Keyboard(p []KeyPacket) error {
	if p[0].Up {
		b.attempts.Add(1)
		if b.blocked.Load() {
			return errors.New("desktop unavailable")
		}
		select {
		case b.released <- struct{}{}:
		default:
		}
	}
	return nil
}
func TestHeldKeyboardTargetLossRetriesReleaseWithoutFurtherInput(t *testing.T) {
	var unavailable atomic.Bool
	b := &keyboardLifecycleBackend{released: make(chan struct{}, 1)}
	c := NewExtended(func() (Target, error) {
		if unavailable.Load() {
			return Target{}, errors.New("secure desktop")
		}
		return target(), nil
	}, b)
	defer c.Close()
	if err := c.HandleKey(remoteinput.Key{Kind: "down", Code: "Control"}); err != nil {
		t.Fatal(err)
	}
	b.blocked.Store(true)
	unavailable.Store(true)
	deadline := time.Now().Add(time.Second)
	for b.attempts.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if b.attempts.Load() == 0 {
		t.Fatal("held keyboard not monitored without touch")
	}
	b.blocked.Store(false)
	unavailable.Store(false)
	select {
	case <-b.released:
	case <-time.After(time.Second):
		t.Fatal("failed release never retried")
	}
	if err := c.HandleKey(remoteinput.Key{Kind: "text", Text: "x"}); err == nil {
		t.Fatal("stale keyboard replayed without resetting controller")
	}
}

func TestRetryReleaseHandlesStickyFailureBeforeClosing(t *testing.T) {
	b := &keyboardLifecycleBackend{released: make(chan struct{}, 1)}
	c := NewExtended(func() (Target, error) { return target(), nil }, b)
	defer c.Close()
	if e := c.HandleKey(remoteinput.Key{Kind: "down", Code: "Control"}); e != nil {
		t.Fatal(e)
	}
	b.blocked.Store(true)
	if e := c.CancelActive(); e == nil {
		t.Fatal("failed release hidden")
	}
	b.blocked.Store(false)
	if e := c.RetryRelease(); e != nil {
		t.Fatal("sticky failure prevented release retry", e)
	}
}
