package wintray

import (
	"errors"
	"testing"
)

func TestStopDuringReconnectStillRetiresPreparedAdapter(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	calls := 0
	err := finishReconnectLoop(stop, nil, true, func() error { calls++; return nil })
	if err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	failure := errors.New("cleanup failed")
	if got := finishReconnectLoop(stop, nil, true, func() error { return failure }); !errors.Is(got, failure) {
		t.Fatal(got)
	}
}
func TestReconnectFailureAndMirrorDoNotRetireAdapterAgain(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	failure := errors.New("display failed")
	forbidden := func() error { t.Fatal("unexpected adapter operation"); return nil }
	if finishReconnectLoop(stop, failure, true, forbidden) != failure {
		t.Fatal("failure hidden")
	}
	if finishReconnectLoop(stop, nil, false, forbidden) != nil {
		t.Fatal("mirror cleanup")
	}
	if finishReconnectLoop(make(chan struct{}), nil, true, forbidden) != nil {
		t.Fatal("not stopped")
	}
}
