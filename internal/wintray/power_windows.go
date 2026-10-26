//go:build windows

package wintray

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
)

// AcquireAwake holds a per-thread Windows display/system sleep request until
// release is called. It changes no persistent power policy.
func AcquireAwake() (func(), error) {
	ready := make(chan error, 1)
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		p := syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadExecutionState")
		r, _, e := p.Call(0x80000003) // CONTINUOUS | SYSTEM_REQUIRED | DISPLAY_REQUIRED
		if r == 0 {
			ready <- fmt.Errorf("mantener pantalla encendida: %w", e)
			return
		}
		ready <- nil
		<-stop
		p.Call(0x80000000) // clear only this thread's request
	}()
	if e := <-ready; e != nil {
		<-done
		return nil, e
	}
	var once sync.Once
	return func() { once.Do(func() { close(stop); <-done }) }, nil
}
