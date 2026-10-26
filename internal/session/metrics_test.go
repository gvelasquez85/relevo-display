package session

import (
	"testing"
	"time"
)

func TestRecentTimingsP95AndEviction(t *testing.T) {
	var r recentTimings
	if r.p95() != 0 {
		t.Fatal("empty percentile")
	}
	for i := 1; i <= 100; i++ {
		r.add(time.Duration(i) * time.Millisecond)
	}
	if r.p95() != 95 {
		t.Fatal(r.p95())
	}
	for i := 0; i < 120; i++ {
		r.add(2 * time.Millisecond)
	}
	if r.p95() != 2 || r.count != 120 {
		t.Fatal(r.p95(), r.count)
	}
	r.add(-time.Second)
	if r.p95() != 2 {
		t.Fatal("negative changed window")
	}
}
