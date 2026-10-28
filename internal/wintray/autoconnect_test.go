package wintray

import (
	"errors"
	"testing"
	"time"
)

func TestAutoConnectWaitsForUSBAndReconnects(t *testing.T) {
	stop := make(chan struct{})
	calls := 0
	err := autoConnect(stop, func(string) {}, func() error {
		calls++
		if calls == 1 {
			return ErrWaitingForDevice
		}
		if calls == 2 {
			return nil
		}
		close(stop)
		return nil
	}, time.Millisecond)
	if err != nil || calls != 3 {
		t.Fatal(err, calls)
	}
}
func TestAutoConnectDoesNotRetryDisplayFailure(t *testing.T) {
	failure := errors.New("display activation failed")
	calls := 0
	err := autoConnect(make(chan struct{}), func(string) {}, func() error { calls++; return failure }, time.Millisecond)
	if !errors.Is(err, failure) || calls != 1 {
		t.Fatal(err, calls)
	}
}
func TestAutoConnectStopInterruptsWait(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- autoConnect(stop, func(string) { close(stop) }, func() error { return ErrWaitingForDevice }, time.Hour)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop blocked")
	}
}

func TestAutoStartRequiresCompatibleUSBExtension(t *testing.T) {
	s := DefaultSettings()
	s.AutoStart = true
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	s.Mode = "desktop"
	if s.Validate() == nil {
		t.Fatal("startup without virtual extension")
	}
	s.Mode = "extend"
	s.Client = "relevo"
	if s.Validate() == nil {
		t.Fatal("startup enabled for unvalidated receiver")
	}
}
