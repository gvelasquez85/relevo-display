//go:build windows

package winplayback

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var winmm = syscall.NewLazyDLL("winmm.dll")
var waveOpen = winmm.NewProc("waveOutOpen")
var wavePrepare = winmm.NewProc("waveOutPrepareHeader")
var waveWrite = winmm.NewProc("waveOutWrite")
var waveReset = winmm.NewProc("waveOutReset")
var waveUnprepare = winmm.NewProc("waveOutUnprepareHeader")
var waveClose = winmm.NewProc("waveOutClose")

type waveFormat struct {
	Tag, Channels        uint16
	Rate, BytesPerSecond uint32
	Align, Bits, Extra   uint16
}
type waveHeader struct {
	Data             *byte
	Length, Recorded uint32
	User             uintptr
	Flags, Loops     uint32
	Next, Reserved   uintptr
}
type waveBuffer struct {
	header   waveHeader
	data     [882]byte
	prepared bool
}

var failedMonitorMu sync.Mutex
var failedMonitors []*waveSink

type waveSink struct {
	pin     runtime.Pinner
	handle  uintptr
	buffers [6]waveBuffer
}

func mmError(op string, r uintptr) error {
	if r != 0 {
		return fmt.Errorf("%s failed (%d)", op, r)
	}
	return nil
}
func nativeBackend() (Backend, error) {
	s := &waveSink{}
	f := waveFormat{Tag: 1, Channels: 1, Rate: 22050, BytesPerSecond: 44100, Align: 2, Bits: 16}
	r, _, _ := waveOpen.Call(uintptr(unsafe.Pointer(&s.handle)), 0xffffffff, uintptr(unsafe.Pointer(&f)), 0, 0, 0)
	runtime.KeepAlive(f)
	if e := mmError("waveOutOpen", r); e != nil {
		return nil, e
	}
	s.pin.Pin(s)
	return s, nil
}
func (s *waveSink) WritePCM(b []byte) error {
	if len(b) < 2 || len(b) > 882 || len(b)%2 != 0 {
		return fmt.Errorf("invalid monitor PCM")
	}
	for i := range s.buffers {
		v := &s.buffers[i]
		if v.prepared && v.header.Flags&1 == 0 {
			continue
		}
		if v.prepared {
			r, _, _ := waveUnprepare.Call(s.handle, uintptr(unsafe.Pointer(&v.header)), unsafe.Sizeof(v.header))
			if e := mmError("waveOutUnprepareHeader", r); e != nil {
				return e
			}
			v.prepared = false
		}
		copy(v.data[:], b)
		v.header = waveHeader{Data: &v.data[0], Length: uint32(len(b))}
		r, _, _ := wavePrepare.Call(s.handle, uintptr(unsafe.Pointer(&v.header)), unsafe.Sizeof(v.header))
		if e := mmError("waveOutPrepareHeader", r); e != nil {
			return e
		}
		v.prepared = true
		r, _, _ = waveWrite.Call(s.handle, uintptr(unsafe.Pointer(&v.header)), unsafe.Sizeof(v.header))
		runtime.KeepAlive(s)
		return mmError("waveOutWrite", r)
	}
	// Six outstanding20ms buffers bound latency to about120ms; drop new audio.
	return nil
}
func (s *waveSink) Close() error {
	if s.handle == 0 {
		return nil
	}
	r, _, _ := waveReset.Call(s.handle)
	result := mmError("waveOutReset", r)
	for i := range s.buffers {
		v := &s.buffers[i]
		if v.prepared {
			r, _, _ = waveUnprepare.Call(s.handle, uintptr(unsafe.Pointer(&v.header)), unsafe.Sizeof(v.header))
			result = errors.Join(result, mmError("waveOutUnprepareHeader", r))
			if r == 0 {
				v.prepared = false
			}
		}
	}
	r, _, _ = waveClose.Call(s.handle)
	result = errors.Join(result, mmError("waveOutClose", r))
	runtime.KeepAlive(s)
	if r == 0 {
		s.handle = 0
		s.pin.Unpin()
	}
	if r != 0 {
		// A driver may still reference prepared headers. Retain pinned storage rather
		// than freeing memory underneath native asynchronous playback after failure.
		failedMonitorMu.Lock()
		failedMonitors = append(failedMonitors, s)
		failedMonitorMu.Unlock()
	}
	return result
}
