package main

import (
	"context"
	"errors"
	"testing"
)

type failingCleanup struct {
	err   error
	calls int
}

func (s *failingCleanup) Next(context.Context) ([]byte, error) { return nil, nil }
func (s *failingCleanup) Close() error                         { s.calls++; return s.err }
func TestCaptureRestoreFailureIsNotReportedAsSuccessfulTrial(t *testing.T) {
	restore := errors.New("virtual display restore failed")
	s := &failingCleanup{err: restore}
	if e := closeCapture(s, nil); !errors.Is(e, restore) || s.calls != 1 {
		t.Fatal(e, s.calls)
	}
}
func TestCaptureAndRestoreFailuresBothSurvive(t *testing.T) {
	capture := errors.New("capture failed")
	restore := errors.New("restore failed")
	e := closeCapture(&failingCleanup{err: restore}, capture)
	if !errors.Is(e, capture) || !errors.Is(e, restore) {
		t.Fatal(e)
	}
	if closeCapture(nil, capture) != capture {
		t.Fatal("lost earlier failure")
	}
}
