package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestOnlyOwnedContextTerminationIsNormal(t *testing.T) {
	if normalizeSessionError(context.Background(), context.DeadlineExceeded) == nil {
		t.Fatal("capture deadline hidden")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if normalizeSessionError(ctx, context.Canceled) != nil {
		t.Fatal("user stop should be normal")
	}
	if normalizeSessionError(ctx, errors.New("restore failed")) == nil {
		t.Fatal("unrelated error hidden")
	}
}

func TestStopPreservesRollbackFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	failure := errors.New("rollback failed")
	mixed := errors.Join(context.Canceled, failure)
	if got := normalizeSessionError(ctx, mixed); !errors.Is(got, failure) {
		t.Fatal("rollback failure hidden")
	}
	if got := normalizeSessionError(ctx, fmt.Errorf("activation: %w", errors.Join(context.Canceled))); got != nil {
		t.Fatalf("safe cancellation: %v", got)
	}
}
