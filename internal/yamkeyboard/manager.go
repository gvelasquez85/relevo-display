// Package yamkeyboard opens a host-rendered keyboard only after a successful
// Yam tap on an editable control or an explicit GUI request. No text is retained.
package yamkeyboard

import (
	"math"
	"relevo/internal/wininput"
	"sync"
	"sync/atomic"
	"time"
)

type request struct {
	x, y       float64
	manual     bool
	generation uint64
}
type Manager struct {
	target     func() (wininput.Target, error)
	probe      func(int32, int32) (bool, error)
	show       func(wininput.Target) (func(), error)
	report     func(string)
	requests   chan request
	stop, done chan struct{}
	once       sync.Once
	closed     atomic.Bool
	generation atomic.Uint64
}

func New(target func() (wininput.Target, error), probe func(int32, int32) (bool, error), show func(wininput.Target) (func(), error), report func(string)) *Manager {
	m := &Manager{target: target, probe: probe, show: show, report: report, requests: make(chan request, 8), stop: make(chan struct{}), done: make(chan struct{})}
	go m.run()
	return m
}
func (m *Manager) Tap(x, y float64) {
	if m.closed.Load() {
		return
	}
	g := m.generation.Add(1)
	select {
	case m.requests <- request{x: x, y: y, generation: g}:
	default:
	}
}
func (m *Manager) Manual() {
	if m.closed.Load() {
		return
	}
	g := m.generation.Add(1)
	select {
	case m.requests <- request{manual: true, generation: g}:
	default:
	}
}
func (m *Manager) Cancel() { m.generation.Add(1) }
func (m *Manager) Close()  { m.once.Do(func() { m.closed.Store(true); close(m.stop) }); <-m.done }
func (m *Manager) run() {
	defer close(m.done)
	var panelClose func()
	defer func() {
		if panelClose != nil {
			panelClose()
		}
	}()
	for {
		select {
		case <-m.stop:
			return
		case r := <-m.requests:
			if !r.manual {
				timer := time.NewTimer(80 * time.Millisecond)
				select {
				case <-m.stop:
					timer.Stop()
					return
				case <-timer.C:
				}
			}
			if m.closed.Load() || r.generation != m.generation.Load() {
				continue
			}
			if m.target == nil || m.show == nil {
				continue
			}
			t, err := m.target()
			if err != nil {
				continue
			}
			if !r.manual {
				ax, ay, e := wininput.Coordinates(t, r.x, r.y)
				if e != nil {
					continue
				}
				x := int32(math.Round(float64(t.Virtual.Min.X) + float64(ax)*float64(t.Virtual.Dx()-1)/65535))
				y := int32(math.Round(float64(t.Virtual.Min.Y) + float64(ay)*float64(t.Virtual.Dy()-1)/65535))
				if m.probe == nil {
					continue
				}
				editable, e := m.probe(x, y)
				if e != nil || !editable {
					continue
				}
			}
			if m.closed.Load() || r.generation != m.generation.Load() {
				continue
			}
			if panelClose != nil {
				panelClose()
				panelClose = nil
			}
			panelClose, err = m.show(t)
			if m.report != nil {
				if err != nil {
					m.report("No se pudo abrir el teclado de Relevo; el vídeo continúa.")
				} else {
					m.report("Teclado Relevo abierto en la pantalla del iPad. Toca el campo donde quieres escribir; Cerrar oculta el teclado.")
				}
			}
		}
	}
}
