package engine

import (
	"math"
	"sync/atomic"
	"time"
)

// histBuckets are fixed upper-bound boundaries in nanoseconds. A sample lands in
// the first bucket whose boundary is >= the sample; samples above the last
// boundary land in a final overflow bucket. Boundaries are chosen to give useful
// resolution across the realistic latency range (sub-millisecond to tens of
// seconds) without a dependency on an HDR histogram.
var histBuckets = [...]time.Duration{
	100 * time.Microsecond,
	250 * time.Microsecond,
	500 * time.Microsecond,
	1 * time.Millisecond,
	2 * time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
}

// histogram is a fixed-bucket, mergeable latency histogram. The zero value is
// ready to use. Counts are atomic.Uint64 so multiple worker goroutines may call
// record() concurrently and snapshot() may call merge() concurrently with them.
// len(counts) == len(histBuckets)+1 (the last slot is the overflow bucket).
type histogram struct {
	counts [len(histBuckets) + 1]atomic.Uint64
}

// record adds one sample. Negative durations are clamped to zero.
func (h *histogram) record(d time.Duration) {
	if d < 0 {
		d = 0
	}
	for i, b := range histBuckets {
		if d <= b {
			h.counts[i].Add(1)
			return
		}
	}
	h.counts[len(histBuckets)].Add(1) // overflow bucket
}

// merge folds src into h without mutating src.
func (h *histogram) merge(src *histogram) {
	for i := range h.counts {
		h.counts[i].Add(src.counts[i].Load())
	}
}

// percentile returns an approximate quantile as the upper boundary of the bucket
// in which the cumulative count crosses q*total. q is clamped to [0,1]. An empty
// histogram returns 0. The overflow bucket reports the final boundary (the best
// fixed-bucket lower bound for outliers).
func (h *histogram) percentile(q float64) time.Duration {
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	var total uint64
	for i := range h.counts {
		total += h.counts[i].Load()
	}
	if total == 0 {
		return 0
	}
	// rank is the count of samples that must fall strictly below the target
	// bucket. Using ceiling of q*total ensures a single-sample outlier at the
	// top of the distribution (e.g. p99 of [100x10ms, 1x5s]) is visible: the
	// 100th sample is exactly at the boundary, so ceil pushes rank past it.
	rank := uint64(math.Ceil(q * float64(total)))
	var cum uint64
	for i := range h.counts {
		cum += h.counts[i].Load()
		if cum > rank {
			if i < len(histBuckets) {
				return histBuckets[i]
			}
			return histBuckets[len(histBuckets)-1] // overflow -> final boundary
		}
	}
	return histBuckets[len(histBuckets)-1]
}
