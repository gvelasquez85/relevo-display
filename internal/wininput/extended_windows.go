//go:build windows

package wininput

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var extendedUser32 = syscall.NewLazyDLL("user32.dll")
var injectTouch = extendedUser32.NewProc("InjectTouchInput")
var sendKeys = extendedUser32.NewProc("SendInput")
var initializeTouch = extendedUser32.NewProc("InitializeTouchInjection")
var touchInitialization sync.Once
var touchInitializationError error
var extendedAPIMu sync.Mutex

type nativeExtended struct{ pending []KeyPacket }

func NativeExtendedBackend() (ExtendedBackend, error) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		return nil, fmt.Errorf("extended input requires Windows 64-bit")
	}
	for _, p := range []*syscall.LazyProc{injectTouch, sendKeys, initializeTouch} {
		if err := p.Find(); err != nil {
			return nil, fmt.Errorf("extended input API unavailable: %w", err)
		}
	}
	touchInitialization.Do(func() {
		ok, _, e := initializeTouch.Call(10, 3)
		if ok == 0 {
			touchInitializationError = fmt.Errorf("InitializeTouchInjection: %v", e)
		}
	})
	if touchInitializationError != nil {
		return nil, touchInitializationError
	}
	return &nativeExtended{}, nil
}
func withInputThread(fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dpi := extendedUser32.NewProc("SetThreadDpiAwarenessContext")
	if dpi.Find() == nil {
		old, _, _ := dpi.Call(^uintptr(3))
		if old != 0 {
			defer dpi.Call(old)
		}
	}
	return fn()
}
func (*nativeExtended) Touch(packets []TouchPacket) error {
	if len(packets) < 1 || len(packets) > 10 {
		return fmt.Errorf("invalid injection contact count")
	}
	infos := make([]winTouchInfo, len(packets))
	seen := make(map[uint32]bool, len(packets))
	for i, p := range packets {
		if p.ID < 1 || p.ID > 10 || seen[p.ID] {
			return fmt.Errorf("invalid injection contact ID")
		}
		seen[p.ID] = true
		infos[i] = touchInfo(p)
		if infos[i].Pointer.Flags == 0 {
			return fmt.Errorf("invalid injection phase")
		}
	}
	extendedAPIMu.Lock()
	defer extendedAPIMu.Unlock()
	return withInputThread(func() error {
		for retry := 0; retry < 3; retry++ {
			ok, _, e := injectTouch.Call(uintptr(len(infos)), uintptr(unsafe.Pointer(&infos[0])))
			runtime.KeepAlive(infos)
			if ok != 0 {
				return nil
			}
			// Automatic timestamps can collide within 0.1ms. Microsoft requires the
			// same frame be retried for ERROR_NOT_READY; other errors end the gesture.
			if e == syscall.Errno(21) && retry < 2 {
				time.Sleep(time.Millisecond)
				continue
			}
			return fmt.Errorf("InjectTouchInput rejected (desktop or integrity boundary): %v", e)
		}
		return nil
	})
}
func (b *nativeExtended) Keyboard(packets []KeyPacket) error {
	if len(packets) == 0 || len(packets) > 8192 {
		return fmt.Errorf("invalid keyboard batch size")
	}
	inputs := make([]winKeyboardInput, len(packets))
	for i, p := range packets {
		inputs[i] = keyboardInfo(p)
	}
	extendedAPIMu.Lock()
	defer extendedAPIMu.Unlock()
	return withInputThread(func() error {
		if e := b.releasePending(); e != nil {
			return e
		}
		n, _, e := sendKeys.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
		runtime.KeepAlive(inputs)
		if n == uintptr(len(inputs)) {
			return nil
		}
		if n > uintptr(len(packets)) {
			n = uintptr(len(packets))
		}
		failure := fmt.Errorf("SendInput rejected or partially applied (desktop or integrity boundary): %v", e)
		releases := keyReleases(packets[:int(n)])
		// Separate cleanup calls ensure an error releasing one key doesn't skip the
		// rest. The controller also retains ordinary held keys for Close retry.
		for _, p := range releases {
			k := keyboardInfo(p)
			done, _, e := sendKeys.Call(1, uintptr(unsafe.Pointer(&k)), unsafe.Sizeof(k))
			if done != 1 {
				b.pending = append(b.pending, p)
				failure = errors.Join(failure, fmt.Errorf("SendInput cleanup rejected: %v", e))
			}
		}
		return failure
	})
}

// Release retries Unicode units left down by a partially failed SendInput call.
// The controller invokes it on error/Close even when no ordinary key is held.
func (b *nativeExtended) Release() error {
	extendedAPIMu.Lock()
	defer extendedAPIMu.Unlock()
	return withInputThread(b.releasePending)
}
func (b *nativeExtended) releasePending() error {
	var failure error
	remaining := make([]KeyPacket, 0, len(b.pending))
	for _, p := range b.pending {
		k := keyboardInfo(p)
		done, _, e := sendKeys.Call(1, uintptr(unsafe.Pointer(&k)), unsafe.Sizeof(k))
		if done != 1 {
			remaining = append(remaining, p)
			failure = errors.Join(failure, fmt.Errorf("SendInput pending cleanup rejected: %v", e))
		}
	}
	b.pending = remaining
	return failure
}
