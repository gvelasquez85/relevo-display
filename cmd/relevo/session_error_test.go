package main

import (
	"context"
	"errors"
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
