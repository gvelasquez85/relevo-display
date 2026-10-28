package wintray

import (
	"errors"
	"testing"
	"time"
)

func TestStartupRetryIsBoundedAndDoesNotChangeManualErrors(t *testing.T) {
	t0 := time.Unix(1, 0)
	failure := errors.New("display not initialized")
	if !errors.Is(startupAttempt(failure, true, t0, t0.Add(time.Second)), errWaitingForDesktop) {
		t.Fatal("handoff failed instead of waiting")
	}
	if !errors.Is(startupAttempt(failure, true, t0, t0.Add(30*time.Second)), failure) {
		t.Fatal("permanent failure hidden")
	}
	if startupAttempt(failure, false, t0, t0) != failure {
		t.Fatal("manual session changed")
	}
	if startupAttempt(nil, true, t0, t0) != nil {
		t.Fatal("successful startup changed")
	}
}
