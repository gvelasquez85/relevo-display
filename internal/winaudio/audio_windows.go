//go:build windows && amd64

package winaudio

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

type guid struct {
	A    uint32
	B, C uint16
	D    [8]byte
}

var clsEnumerator = guid{0xbcde0395, 0xe52f, 0x467c, [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}}
var iidEnumerator = guid{0xa95664d2, 0x9614, 0x4f35, [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}}
var iidClient = guid{0x1cb9ad4c, 0xdbfa, 0x4c32, [8]byte{0xb1, 0x78, 0xc2, 0xf5, 0x68, 0xa7, 0x03, 0xb2}}
var iidCapture = guid{0xc8adbd64, 0xe71e, 0x48a0, [8]byte{0xa4, 0xde, 0x18, 0x5c, 0x39, 0x5c, 0xd3, 0x17}}

func invoke(p unsafe.Pointer, index int, args ...uintptr) error {
	vt := *(*unsafe.Pointer)(p)
	fn := *(*uintptr)(unsafe.Add(vt, index*int(unsafe.Sizeof(uintptr(0)))))
	a := append([]uintptr{uintptr(p)}, args...)
	hr, _, _ := syscall.SyscallN(fn, a...)
	if int32(hr) < 0 {
		return fmt.Errorf("WASAPI COM method %d HRESULT 0x%08x", index, uint32(hr))
	}
	return nil
}
func release(p unsafe.Pointer) {
	if p != nil {
		invoke(p, 2)
	}
}
func Open(ctx context.Context) (Source, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &source{packets: make(chan []byte, 6), cancel: cancel, done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(s.done)
		defer close(s.packets)
		e := s.capture(ctx, ready)
		s.mu.Lock()
		s.err = e
		s.mu.Unlock()
	}()
	if e := <-ready; e != nil {
		cancel()
		<-s.done
		return nil, e
	}
	return s, nil
}
func (s *source) capture(ctx context.Context, ready chan<- error) (err error) {
	announced := false
	defer func() {
		if !announced {
			ready <- err
		}
	}()
	ole := syscall.NewLazyDLL("ole32.dll")
	init := ole.NewProc("CoInitializeEx")
	uninit := ole.NewProc("CoUninitialize")
	create := ole.NewProc("CoCreateInstance")
	free := ole.NewProc("CoTaskMemFree")
	hr, _, _ := init.Call(0, 0)
	if int32(hr) < 0 {
		return fmt.Errorf("CoInitializeEx 0x%x", hr)
	}
	defer uninit.Call()
	var enumerator, device, client, capture unsafe.Pointer
	hr, _, _ = create.Call(uintptr(unsafe.Pointer(&clsEnumerator)), 0, 23, uintptr(unsafe.Pointer(&iidEnumerator)), uintptr(unsafe.Pointer(&enumerator)))
	if int32(hr) < 0 {
		return fmt.Errorf("MMDeviceEnumerator 0x%x", hr)
	}
	defer release(enumerator)
	if err = invoke(enumerator, 4, 0, 1, uintptr(unsafe.Pointer(&device))); err != nil {
		return
	}
	defer release(device)
	if err = invoke(device, 3, uintptr(unsafe.Pointer(&iidClient)), 23, 0, uintptr(unsafe.Pointer(&client))); err != nil {
		return
	}
	defer release(client)
	var format unsafe.Pointer
	if err = invoke(client, 8, uintptr(unsafe.Pointer(&format))); err != nil {
		return
	}
	defer free.Call(uintptr(format))
	f := unsafe.Slice((*byte)(format), 18)
	tag := binary.LittleEndian.Uint16(f)
	channels := int(binary.LittleEndian.Uint16(f[2:]))
	rate := int(binary.LittleEndian.Uint32(f[4:]))
	align := int(binary.LittleEndian.Uint16(f[12:]))
	bits := int(binary.LittleEndian.Uint16(f[14:]))
	if tag == 0xfffe {
		if binary.LittleEndian.Uint16(f[16:]) < 22 {
			return fmt.Errorf("short extensible mix format")
		}
		x := unsafe.Slice((*byte)(format), 40)
		tag = uint16(binary.LittleEndian.Uint32(x[24:]))
	}
	if channels < 1 || channels > 32 || rate < 8000 || rate > 384000 || align != channels*(bits/8) || !((tag == 3 && bits == 32) || (tag == 1 && (bits == 16 || bits == 24 || bits == 32))) {
		return fmt.Errorf("unsupported WASAPI mix format tag=%d bits=%d channels=%d rate=%d", tag, bits, channels, rate)
	}
	if err = invoke(client, 3, 0, 0x20000, 1000000, 0, uintptr(format), 0); err != nil {
		return
	}
	if err = invoke(client, 14, uintptr(unsafe.Pointer(&iidCapture)), uintptr(unsafe.Pointer(&capture))); err != nil {
		return
	}
	defer release(capture)
	if err = invoke(client, 10); err != nil {
		return
	}
	defer invoke(client, 11)
	ready <- nil
	announced = true
	r := resampler{rate: rate}
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
		for {
			var count uint32
			if err = invoke(capture, 5, uintptr(unsafe.Pointer(&count))); err != nil {
				return
			}
			if count == 0 {
				break
			}
			var data unsafe.Pointer
			var frames, flags uint32
			if err = invoke(capture, 3, uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&frames)), uintptr(unsafe.Pointer(&flags)), 0, 0); err != nil {
				return
			}
			if frames > uint32(rate*2) {
				invoke(capture, 4, uintptr(frames))
				return fmt.Errorf("WASAPI oversized packet")
			}
			samples := make([]float64, int(frames))
			if flags&2 == 0 {
				raw := unsafe.Slice((*byte)(data), int(frames)*align)
				for i := range samples {
					for ch := 0; ch < channels; ch++ {
						p := raw[i*align+ch*(bits/8):]
						var v float64
						if tag == 3 {
							v = float64(math.Float32frombits(binary.LittleEndian.Uint32(p)))
						} else {
							switch bits {
							case 16:
								v = float64(int16(binary.LittleEndian.Uint16(p))) / 32768
							case 24:
								n := int32(p[0]) | int32(p[1])<<8 | int32(p[2])<<16
								n = n << 8 >> 8
								v = float64(n) / 8388608
							case 32:
								v = float64(int32(binary.LittleEndian.Uint32(p))) / 2147483648
							}
						}
						samples[i] += v / float64(channels)
					}
				}
			}
			if err = invoke(capture, 4, uintptr(frames)); err != nil {
				return
			}
			for _, b := range r.feed(samples) {
				select {
				case s.packets <- b:
				default:
					select {
					case <-s.packets:
					default:
					}
					select {
					case s.packets <- b:
					default:
					}
				}
			}
		}
	}
}
