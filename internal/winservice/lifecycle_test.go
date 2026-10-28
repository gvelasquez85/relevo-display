package winservice

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPrepareRetriesStartupThenReady(t *testing.T) {
	calls := 0
	err := prepareWithRetry(context.Background(), func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("not ready")
		}
		return nil
	}, time.Millisecond)
	if err != nil || calls != 3 {
		t.Fatal(calls, err)
	}
}
func TestPrepareCancellationDoesNotWaitForRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := prepareWithRetry(ctx, func(context.Context) error { calls++; cancel(); return errors.New("not ready") }, time.Hour)
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal(calls, err)
	}
}
func TestPrepareNeverInvokesCallbackAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := prepareWithRetry(ctx, func(context.Context) error { calls++; return nil }, time.Millisecond)
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal(calls, err)
	}
}
func TestPrepareTimeoutReportsLastFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	failure := errors.New("adapter not enumerated")
	err := prepareWithRetry(ctx, func(context.Context) error { return failure }, time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, failure) {
		t.Fatal(err)
	}
}

func TestConsoleBrokerOnlyBeforeLogin(t *testing.T) {
	for _, c := range []struct {
		session uint32
		noUser  bool
		err     error
		want    bool
	}{
		{1, true, nil, true}, {0, true, nil, false}, {0xffffffff, true, nil, false},
		{1, false, nil, false}, {1, true, errors.New("access denied"), false},
	} {
		if got := eligibleConsole(c.session, c.noUser, c.err); got != c.want {
			t.Fatal(c, got)
		}
	}
}
