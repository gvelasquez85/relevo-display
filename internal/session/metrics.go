package session

import (
	"sort"
	"time"
)

// Recent timings bound memory during long display sessions. They are sender
// timings, not presentation latency or an estimate of the iPad's display rate.
type recentTimings struct {
	values      [120]float64
	next, count int
}

func (r *recentTimings) add(d time.Duration) {
	if d < 0 {
		return
	}
	r.values[r.next] = float64(d) / float64(time.Millisecond)
	r.next = (r.next + 1) % len(r.values)
	if r.count < len(r.values) {
		r.count++
	}
}
func (r *recentTimings) p95() float64 {
	if r.count == 0 {
		return 0
	}
	v := append([]float64(nil), r.values[:r.count]...)
	sort.Float64s(v)
	// Nearest-rank percentile, ceil(95*n/100)-1.
	return v[(95*len(v)+99)/100-1]
}
