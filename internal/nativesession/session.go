// Package nativesession serves one explicitly started native iPad session.
package nativesession

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"math"
	"net"
	"relevo/internal/nativewire"
	"relevo/internal/remoteinput"
	"relevo/internal/wininput"
	"sync"
	"time"
)

type Options struct {
	Progress                                              func(Stats)
	InputModesEnabled, InputFocusEnabled, PointerEnabled  bool
	EditableAt                                            func(float64, float64) bool
	OnMode                                                func(string) error
	OnPointer                                             func(wininput.PointerEvent) error
	MultiTouchEnabled, KeyboardEnabled, MicrophoneEnabled bool
	OnContacts                                            func([]remoteinput.Contact) error
	OnKey                                                 func(remoteinput.Key) error
	Microphone                                            func([]byte) error
	FPS                                                   int
	Next                                                  func(context.Context) ([]byte, error)
	TouchEnabled, AudioEnabled, CameraEnabled             bool
	OnTouch                                               func(TouchEvent) error
	Audio                                                 <-chan []byte
	Camera                                                func([]byte) error
}
type TouchEvent struct {
	Phase string  `json:"phase"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}
type Stats struct {
	MicrophonePackets uint64
	KeyboardEvents    uint64
	Frames            uint64
	Touches           uint64
}

func Run(ctx context.Context, c net.Conn, o Options) (s Stats, err error) {
	if o.FPS < 1 || o.FPS > 60 || o.Next == nil {
		return s, fmt.Errorf("invalid session options")
	}
	defer c.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			c.Close()
		case <-done:
		}
	}()
	write := func(f nativewire.Frame) error {
		c.SetWriteDeadline(time.Now().Add(3 * time.Second))
		return nativewire.Write(c, f)
	}
	read := func() (nativewire.Frame, error) {
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		return nativewire.Read(c)
	}
	active := false
	mode := "screen"
	var taps tapTracker
	focusReply := func(seq uint32, editable bool) error {
		b, _ := json.Marshal(map[string]any{"editable": editable, "sequence": seq})
		return write(nativewire.Frame{Type: nativewire.InputFocus, Sequence: seq, Payload: b})
	}
	var touchState remoteinput.State
	var keyState remoteinput.KeyboardState
	control := func(f nativewire.Frame) error {
		switch f.Type {
		case nativewire.Ping:
			return write(nativewire.Frame{Type: nativewire.Pong, Sequence: f.Sequence})
		case nativewire.Pong:
			return nil
		case nativewire.InputMode:
			if !active || !o.InputModesEnabled {
				return fmt.Errorf("input modes not negotiated")
			}
			var m struct {
				Mode string `json:"mode"`
			}
			if e := strictJSON(f.Payload, &m); e != nil {
				return e
			}
			if m.Mode != "screen" && m.Mode != "keyboard" && m.Mode != "pointer" {
				return fmt.Errorf("invalid input mode")
			}
			if m.Mode == "keyboard" && !o.KeyboardEnabled || m.Mode == "pointer" && !o.PointerEnabled {
				return fmt.Errorf("input mode capability not negotiated")
			}
			if o.OnMode != nil {
				if e := o.OnMode(m.Mode); e != nil {
					return e
				}
			}
			mode = m.Mode
			touchState.Cancel()
			keyState.Release()
			taps = tapTracker{}
			return nil
		case nativewire.Pointer:
			if !active || !o.PointerEnabled || mode != "pointer" {
				return fmt.Errorf("pointer not negotiated or active")
			}
			var p wininput.PointerEvent
			if e := strictJSON(f.Payload, &p); e != nil {
				return e
			}
			if e := p.Validate(); e != nil {
				return e
			}
			if o.OnPointer != nil {
				return o.OnPointer(p)
			}
			return nil
		case nativewire.MultiTouch:
			if !active || !o.MultiTouchEnabled || mode != "screen" {
				return fmt.Errorf("multitouch not negotiated")
			}
			contacts, e := decodeContacts(f.Payload)
			if e != nil {
				return e
			}
			if e = touchState.Apply(contacts); e != nil {
				return e
			}
			if o.OnContacts != nil {
				if e := o.OnContacts(contacts); e != nil {
					return e
				}
			}
			s.Touches += uint64(len(contacts))
			if o.InputFocusEnabled && o.EditableAt != nil {
				if x, y, ok := taps.apply(contacts); ok {
					return focusReply(f.Sequence, o.EditableAt(x, y))
				}
			}
			return nil
		case nativewire.Keyboard:
			if !active || !o.KeyboardEnabled {
				return fmt.Errorf("keyboard not negotiated")
			}
			key, e := decodeKey(f.Payload)
			if e != nil {
				return e
			}
			if e = keyState.Apply(key); e != nil {
				return e
			}
			if o.OnKey != nil {
				if e := o.OnKey(key); e != nil {
					return e
				}
			}
			s.KeyboardEvents++
			return nil
		case nativewire.MicrophonePCM:
			if !active || !o.MicrophoneEnabled {
				return fmt.Errorf("microphone not negotiated")
			}
			if e := nativewire.ValidatePCM(f.Payload); e != nil {
				return e
			}
			if o.Microphone != nil {
				if e := o.Microphone(f.Payload); e != nil {
					return e
				}
			}
			s.MicrophonePackets++
			return nil
		case nativewire.Touch:
			if !active {
				return fmt.Errorf("touch before ready")
			}
			var t struct {
				Phase string   `json:"phase"`
				X     *float64 `json:"x"`
				Y     *float64 `json:"y"`
			}
			if e := json.Unmarshal(f.Payload, &t); e != nil {
				return e
			}
			if t.Phase != "down" && t.Phase != "move" && t.Phase != "up" && t.Phase != "cancel" {
				return fmt.Errorf("unknown touch phase")
			}
			if t.X == nil || t.Y == nil || math.IsNaN(*t.X) || math.IsNaN(*t.Y) || math.IsInf(*t.X, 0) || math.IsInf(*t.Y, 0) || *t.X < 0 || *t.X > 1 || *t.Y < 0 || *t.Y > 1 {
				return fmt.Errorf("invalid touch coordinates")
			}
			if o.TouchEnabled && o.OnTouch != nil {
				if e := o.OnTouch(TouchEvent{t.Phase, *t.X, *t.Y}); e != nil {
					return e
				}
			}
			s.Touches++
			if mode == "screen" && o.TouchEnabled && o.InputFocusEnabled && o.EditableAt != nil {
				if x, y, ok := taps.apply([]remoteinput.Contact{{ID: 1, Phase: t.Phase, X: *t.X, Y: *t.Y}}); ok {
					return focusReply(f.Sequence, o.EditableAt(x, y))
				}
			}
			return nil
		}
		return fmt.Errorf("unexpected frame type %d", f.Type)
	}
	f, e := read()
	if e != nil {
		return s, e
	}
	var hello struct {
		Protocol int `json:"protocol"`
		Width    int `json:"width"`
		Height   int `json:"height"`
	}
	if f.Type != nativewire.Hello {
		return s, fmt.Errorf("hello required")
	}
	if e = json.Unmarshal(f.Payload, &hello); e != nil {
		return s, e
	}
	if hello.Protocol != 1 || hello.Width != 1024 || hello.Height != 768 {
		return s, fmt.Errorf("unsupported client protocol or dimensions")
	}
	b, _ := json.Marshal(map[string]any{"width": 1024, "height": 768, "fps": o.FPS, "touch": o.TouchEnabled, "audio": o.AudioEnabled, "camera": o.CameraEnabled, "multitouch": o.MultiTouchEnabled, "keyboard": o.KeyboardEnabled, "microphone": o.MicrophoneEnabled, "inputModes": o.InputModesEnabled, "inputFocus": o.InputFocusEnabled, "pointer": o.PointerEnabled})
	if e = write(nativewire.Frame{Type: nativewire.Start, Payload: b}); e != nil {
		return s, e
	}
	readyDeadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(readyDeadline) {
			return s, fmt.Errorf("ready timeout")
		}
		f, e = read()
		if e != nil {
			return s, e
		}
		if f.Type == nativewire.Status {
			var status struct {
				State      string `json:"state"`
				Touch      bool   `json:"touch"`
				Audio      bool   `json:"audio"`
				Camera     bool   `json:"camera"`
				MultiTouch bool   `json:"multitouch"`
				Keyboard   bool   `json:"keyboard"`
				Microphone bool   `json:"microphone"`
				InputModes bool   `json:"inputModes"`
				InputFocus bool   `json:"inputFocus"`
				Pointer    bool   `json:"pointer"`
			}
			if e = json.Unmarshal(f.Payload, &status); e != nil {
				return s, e
			}
			if o.TouchEnabled && !status.Touch || o.AudioEnabled && !status.Audio || o.CameraEnabled && !status.Camera || o.MultiTouchEnabled && !status.MultiTouch || o.KeyboardEnabled && !status.Keyboard || o.MicrophoneEnabled && !status.Microphone || o.InputModesEnabled && !status.InputModes || o.InputFocusEnabled && !status.InputFocus || o.PointerEnabled && !status.Pointer {
				return s, fmt.Errorf("requested capability not acknowledged")
			}
			if status.State != "ready" {
				return s, fmt.Errorf("client not ready")
			}
			break
		}
		if e = control(f); e != nil {
			return s, e
		}
	}

	active = true
	type incoming struct {
		f   nativewire.Frame
		err error
	}
	rx := make(chan incoming, 8)
	pumpDone := make(chan struct{})
	defer close(pumpDone)
	go func() {
		for {
			f, e := read()
			select {
			case rx <- incoming{f, e}:
			case <-pumpDone:
				return
			}
			if e != nil {
				return
			}
		}
	}()
	type captured struct {
		b   []byte
		err error
	}
	frames := make(chan captured, 1)
	captureCtx, cancelCapture := context.WithCancel(ctx)
	var captureWG sync.WaitGroup
	defer func() { cancelCapture(); captureWG.Wait() }()
	heartbeat := time.NewTicker(2 * time.Second)
	defer heartbeat.Stop()
	ticker := time.NewTicker(time.Second / time.Duration(o.FPS))
	defer ticker.Stop()
	var pending uint32
	seq := uint32(0)
	capturing := false
	var ackUntil time.Time
	timeout := time.NewTicker(time.Second)
	defer timeout.Stop()
	audio := o.Audio
	if !o.AudioEnabled {
		audio = nil
	}
	for {
		select {
		case <-ctx.Done():
			return s, ctx.Err()
		case <-heartbeat.C:
			if e := write(nativewire.Frame{Type: nativewire.Ping}); e != nil {
				return s, e
			}
		case <-timeout.C:
			if o.Progress != nil {
				o.Progress(s)
			}
			if pending != 0 && time.Now().After(ackUntil) {
				return s, fmt.Errorf("frame acknowledgment timeout")
			}
		case <-ticker.C:
			if mode == "screen" && pending == 0 && !capturing {
				capturing = true
				captureWG.Add(1)
				go func() {
					defer captureWG.Done()
					cc, cancel := context.WithTimeout(captureCtx, 3*time.Second)
					defer cancel()
					b, e := o.Next(cc)
					select {
					case frames <- captured{b, e}:
					case <-captureCtx.Done():
					}
				}()
			}
		case r := <-frames:
			capturing = false
			if mode != "screen" {
				continue
			}
			if r.err != nil {
				return s, r.err
			}
			b = r.b
			if len(b) > 4*1024*1024 {
				return s, fmt.Errorf("source JPEG exceeds limit")
			}
			cfg, e := jpeg.DecodeConfig(bytes.NewReader(b))
			if e != nil || cfg.Width != 1024 || cfg.Height != 768 {
				return s, fmt.Errorf("invalid source JPEG dimensions")
			}
			seq++
			if e = write(nativewire.Frame{Type: nativewire.DisplayJPEG, Sequence: seq, Payload: b}); e != nil {
				return s, e
			}
			pending = seq
			ackUntil = time.Now().Add(5 * time.Second)
		case b, ok := <-audio:
			if !ok {
				if ctx.Err() != nil {
					return s, ctx.Err()
				}
				return s, fmt.Errorf("audio source ended")
			}
			if e := nativewire.ValidatePCM(b); e != nil {
				return s, e
			}
			if e := write(nativewire.Frame{Type: nativewire.AudioPCM, Payload: b}); e != nil {
				return s, e
			}
		case r := <-rx:
			if r.err != nil {
				return s, r.err
			}
			f = r.f
			switch f.Type {
			case nativewire.FrameApplied:
				if pending == 0 || f.Sequence != pending {
					return s, fmt.Errorf("unexpected acknowledgment sequence")
				}
				pending = 0
				s.Frames++
			case nativewire.CameraJPEG:
				if !o.CameraEnabled {
					return s, fmt.Errorf("camera not negotiated")
				}
				cfg, e := jpeg.DecodeConfig(bytes.NewReader(f.Payload))
				if e != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 640 || cfg.Height > 480 {
					return s, fmt.Errorf("invalid camera JPEG")
				}
				if o.Camera != nil {
					if e = o.Camera(f.Payload); e != nil {
						return s, e
					}
				}
			default:
				if e = control(f); e != nil {
					return s, e
				}
			}
		}
	}
}
