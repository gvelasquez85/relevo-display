// Package winaudio captures the default Windows render endpoint, never a microphone.
package winaudio

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
)

type Source interface {
	Packets() <-chan []byte
	Close() error
}
type source struct {
	packets chan []byte
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
	err     error
}

func (s *source) Packets() <-chan []byte { return s.packets }
func (s *source) Close() error           { s.cancel(); <-s.done; s.mu.Lock(); defer s.mu.Unlock(); return s.err }

// resampler keeps fractional position across input packets. Linear interpolation limits CPU use.
type resampler struct {
	rate    int
	samples []float64
	pos     float64
	pcm     []byte
}

func (r *resampler) feed(in []float64) [][]byte {
	r.samples = append(r.samples, in...)
	var out [][]byte
	step := float64(r.rate) / 22050
	for int(r.pos)+1 < len(r.samples) {
		i := int(r.pos)
		f := r.pos - float64(i)
		v := r.samples[i]*(1-f) + r.samples[i+1]*f
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = 0
		}
		v = math.Max(-1, math.Min(1, v))
		n := int16(math.Round(v * 32767))
		r.pcm = append(r.pcm, byte(n), byte(uint16(n)>>8))
		r.pos += step
		if len(r.pcm) == 882 {
			b := make([]byte, 890)
			binary.LittleEndian.PutUint32(b, 22050)
			binary.LittleEndian.PutUint16(b[4:], 1)
			binary.LittleEndian.PutUint16(b[6:], 16)
			copy(b[8:], r.pcm)
			out = append(out, b)
			r.pcm = r.pcm[:0]
		}
	}
	n := int(r.pos)
	if n > len(r.samples) {
		n = len(r.samples)
	}
	r.samples = append(r.samples[:0], r.samples[n:]...)
	r.pos -= float64(n)
	return out
}
