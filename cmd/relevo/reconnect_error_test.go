package main

import (
	"errors"
	"relevo/internal/wintray"
	"relevo/internal/winvirtual"
	"testing"
)

func TestBackgroundReconnectAllowsPreservedSoleDisplay(t *testing.T) {
	suspended := errors.New("app suspended")
	keep := &winvirtual.KeepDisplayActiveError{}
	if err := compatibleSessionEnd(suspended, keep, true); !errors.Is(err, wintray.ErrWaitingForDevice) {
		t.Fatal(err)
	}
	if err := compatibleSessionEnd(suspended, nil, true); !errors.Is(err, wintray.ErrWaitingForDevice) {
		t.Fatal(err)
	}
}
func TestReconnectDoesNotHideIndependentCleanupFailure(t *testing.T) {
	release := errors.New("ownership release failed")
	keep := &winvirtual.KeepDisplayActiveError{}
	err := compatibleSessionEnd(errors.New("suspended"), errors.Join(keep, release), true)
	if !errors.Is(err, release) || errors.Is(err, wintray.ErrWaitingForDevice) {
		t.Fatal(err)
	}
	if errors.Is(compatibleSessionEnd(errors.New("stop"), keep, false), wintray.ErrWaitingForDevice) {
		t.Fatal("stop retried")
	}
}
