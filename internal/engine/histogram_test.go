package engine

import (
	"testing"
	"time"
)

func TestHistogramPercentileEmpty(t *testing.T) {
	t.Parallel()
	var h histogram
	if got := h.percentile(0.5); got != 0 {
		t.Fatalf("empty p50 = %v, want 0", got)
	}
	if got := h.percentile(0.99); got != 0 {
		t.Fatalf("empty p99 = %v, want 0", got)
	}
}

func TestHistogramRecordPercentile(t *testing.T) {
	t.Parallel()
	var h histogram
	// 100 samples clustered at 10ms, plus one large 5s outlier.
	for i := 0; i < 100; i++ {
		h.record(10 * time.Millisecond)
	}
	h.record(5 * time.Second)

	// p50 must land in the 10ms cluster: a fixed-bucket estimate returns the
	// bucket's upper boundary, so p50 is >= 10ms and well under 1s.
	p50 := h.percentile(0.5)
	if p50 < 10*time.Millisecond || p50 > time.Second {
		t.Fatalf("p50 = %v, want within [10ms, 1s]", p50)
	}
	// p99 must reflect the tail (the 5s outlier sits in the top/overflow bucket).
	p99 := h.percentile(0.99)
	if p99 < p50 {
		t.Fatalf("p99 = %v must be >= p50 = %v", p99, p50)
	}
	if p99 < time.Second {
		t.Fatalf("p99 = %v, want >= 1s (outlier in tail bucket)", p99)
	}
}

func TestHistogramMerge(t *testing.T) {
	t.Parallel()
	var a, b histogram
	for i := 0; i < 10; i++ {
		a.record(5 * time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		b.record(500 * time.Millisecond)
	}

	var dst histogram
	dst.merge(&a)
	dst.merge(&b)

	// Merged set is 10 fast + 10 slow: the median sits at the boundary between
	// the clusters, so p50 is at most the slow cluster and p90 reaches it.
	p90 := dst.percentile(0.9)
	if p90 < 500*time.Millisecond {
		t.Fatalf("merged p90 = %v, want >= 500ms", p90)
	}
	// merge must not mutate the source histograms.
	if a.percentile(0.5) >= 500*time.Millisecond {
		t.Fatalf("merge mutated source a: p50 = %v", a.percentile(0.5))
	}
}

func TestHistogramPercentileClampsQuantile(t *testing.T) {
	t.Parallel()
	var h histogram
	h.record(7 * time.Millisecond)
	// Out-of-range quantiles clamp to [0,1] rather than panicking.
	if got := h.percentile(-1); got <= 0 {
		t.Fatalf("p(-1) = %v, want a positive recorded value", got)
	}
	if got := h.percentile(2); got <= 0 {
		t.Fatalf("p(2) = %v, want a positive recorded value", got)
	}
}
