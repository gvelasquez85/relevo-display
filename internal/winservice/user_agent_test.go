package winservice

import (
	"errors"
	"testing"
	"time"
)

func TestUserAgentStartsOnceAndDoesNotRespawnExplicitExit(t *testing.T) {
	var state userAgentLaunch
	now := time.Unix(1, 0)
	if state.due(1, true, nil, now) {
		t.Fatal("prelogin launched user agent")
	}
	if !state.due(1, false, nil, now) {
		t.Fatal("login did not launch")
	}
	state.result(nil, now)
	if state.due(1, false, nil, now.Add(time.Hour)) {
		t.Fatal("successful launch respawned")
	}
	if state.due(1, true, nil, now) {
		t.Fatal("logoff launched agent")
	}
	if !state.due(1, false, nil, now) {
		t.Fatal("next login not launched")
	}
}
func TestUserAgentLaunchRetriesFailuresAndFailsClosed(t *testing.T) {
	var state userAgentLaunch
	now := time.Unix(1, 0)
	failure := errors.New("launch failed")
	if state.due(0xffffffff, false, nil, now) || state.due(0, false, nil, now) || state.due(1, false, failure, now) {
		t.Fatal("unknown console launched")
	}
	if !state.due(1, false, nil, now) {
		t.Fatal("login not eligible")
	}
	state.result(failure, now)
	if state.due(1, false, nil, now.Add(time.Second)) {
		t.Fatal("unbounded retry")
	}
	if !state.due(1, false, nil, now.Add(2*time.Second)) {
		t.Fatal("launch not retried")
	}
	state.result(nil, now)
	if state.due(1, false, failure, now) || state.due(1, false, nil, now) {
		t.Fatal("query error reset success")
	}
	if !state.due(2, false, nil, now) {
		t.Fatal("new console not launched")
	}
}
