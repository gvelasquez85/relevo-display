// Package yamkeyboard opens a host-rendered keyboard only after a successful
// Yam tap on an editable control or an explicit GUI request. No text is retained.
package yamkeyboard

import (
	"context"
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
	target       func() (wininput.Target, error)
	probe        func(int32, int32) (bool, error)
	show         func(context.Context, wininput.Target) (func(), error)
	ctx          context.Context
	cancel       context.CancelFunc
	workMu       sync.Mutex
	activeCancel context.CancelFunc
	visible      func() bool
	report       func(string)
	requests     chan request
	stop, done   chan struct{}
	once         sync.Once
	closed       atomic.Bool
	generation   atomic.Uint64
}

func New(target func() (wininput.Target, error), probe func(int32, int32) (bool, error), show func(wininput.Target) (func(), error), report func(string), visible ...func() bool) *Manager {
	var contextual func(context.Context, wininput.Target) (func(), error)
	if show != nil {
		contextual = func(_ context.Context, t wininput.Target) (func(), error) { return show(t) }
	}
	return NewWithContext(target, probe, contextual, report, visible...)
}
func NewWithContext(target func() (wininput.Target, error), probe func(int32, int32) (bool, error), show func(context.Context, wininput.Target) (func(), error), report func(string), visible ...func() bool) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{target: target, probe: probe, show: show, report: report, requests: make(chan request, 1), stop: make(chan struct{}), done: make(chan struct{}), ctx: ctx, cancel: cancel}
	if len(visible) > 0 {
		m.visible = visible[0]
	}
	go m.run()
	return m
}
func (m *Manager) enqueue(r request) {
	m.workMu.Lock()
	defer m.workMu.Unlock()
	if m.closed.Load() {
		return
	}
	if m.activeCancel != nil {
		m.activeCancel()
	}
	r.generation = m.generation.Add(1)
	select {
	case <-m.requests:
	default:
	}
	m.requests <- r
}
func (m *Manager) Tap(x, y float64) { m.enqueue(request{x: x, y: y}) }
func (m *Manager) Manual()          { m.enqueue(request{manual: true}) }
func (m *Manager) Cancel() {
	m.workMu.Lock()
	defer m.workMu.Unlock()
	m.generation.Add(1)
	if m.activeCancel != nil {
		m.activeCancel()
	}
}
func (m *Manager) Close() {
	m.once.Do(func() { m.closed.Store(true); m.cancel(); close(m.stop) })
	<-m.done
}
func (m *Manager) begin(r request) (context.Context, context.CancelFunc) {
	m.workMu.Lock()
	defer m.workMu.Unlock()
	ctx, cancel := context.WithCancel(m.ctx)
	if m.closed.Load() || r.generation != m.generation.Load() {
		cancel()
	} else {
		m.activeCancel = cancel
	}
	return ctx, cancel
}
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
			if m.closed.Load() || r.generation != m.generation.Load() {
				continue
			}
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
			if panelClose != nil && m.visible != nil && m.visible() {
				continue
			}
			if panelClose != nil {
				panelClose()
				panelClose = nil
			}
			workCtx, workCancel := m.begin(r)
			if workCtx.Err() != nil {
				workCancel()
				continue
			}
			panelClose, err = m.show(workCtx, t)
			workCancel()
			if m.closed.Load() || r.generation != m.generation.Load() {
				if panelClose != nil {
					panelClose()
					panelClose = nil
				}
				continue
			}
			if m.report != nil {
				if err != nil {
					m.report("No se pudo abrir el teclado de Windows; el vídeo continúa. " + err.Error())
				} else {
					m.report("Teclado de Windows abierto en la pantalla del iPad. Toca el campo donde quieres escribir; cierra su ventana para ocultarlo.")
				}
			}
		}
	}
}
