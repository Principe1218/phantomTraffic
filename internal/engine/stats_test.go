package engine

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
)

func result(target string, o protocols.Outcome, lat time.Duration, in, out int64) protocols.Result {
	return protocols.Result{
		Protocol: protocols.ProtoHTTP,
		Target:   target,
		Outcome:  o,
		Latency:  lat,
		BytesIn:  in,
		BytesOut: out,
	}
}

func TestCollectorRecordAndSnapshot(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	c := newCollector([]string{"t1", "t2"}, clk, func() float64 { return 12.5 })

	c.Record(result("t1", protocols.OutcomeSuccess, 10*time.Millisecond, 100, 20))
	c.Record(result("t1", protocols.OutcomeFailure, 30*time.Millisecond, 0, 0))
	c.Record(result("t2", protocols.OutcomeSuccess, 5*time.Millisecond, 50, 5))
	c.Record(result("t2", protocols.OutcomeReconnect, 1*time.Millisecond, 0, 0))
	c.Record(result("t2", protocols.OutcomePanicked, 2*time.Millisecond, 0, 0))

	snap := c.snapshot()

	// Reconnect and panic are first-class and excluded from success/failure.
	if snap.Requests != 5 {
		t.Fatalf("Requests = %d, want 5", snap.Requests)
	}
	if snap.Successes != 2 {
		t.Fatalf("Successes = %d, want 2", snap.Successes)
	}
	if snap.Failures != 1 {
		t.Fatalf("Failures = %d, want 1", snap.Failures)
	}
	if snap.Reconnects != 1 {
		t.Fatalf("Reconnects = %d, want 1", snap.Reconnects)
	}
	if snap.Panics != 1 {
		t.Fatalf("Panics = %d, want 1", snap.Panics)
	}
	if snap.BytesIn != 150 || snap.BytesOut != 25 {
		t.Fatalf("bytes = (%d,%d), want (150,25)", snap.BytesIn, snap.BytesOut)
	}
	if snap.CapSaturationPct != 12.5 {
		t.Fatalf("CapSaturationPct = %v, want 12.5", snap.CapSaturationPct)
	}
	if !snap.At.Equal(clk.Now()) {
		t.Fatalf("At = %v, want %v", snap.At, clk.Now())
	}

	for target, want := range map[string]TargetStats{
		"t1": {Requests: 2, Successes: 1, Failures: 1, BytesIn: 100, BytesOut: 20},
		"t2": {Requests: 3, Successes: 1, BytesIn: 50, BytesOut: 5},
	} {
		if got := snap.PerTarget[target]; got != want {
			t.Fatalf("PerTarget[%q] = %+v, want %+v", target, got, want)
		}
	}

	// Latency percentiles fold across shards and are non-zero given samples.
	if snap.LatencyP50 <= 0 || snap.LatencyP99 <= 0 {
		t.Fatalf("percentiles p50=%v p99=%v, want >0", snap.LatencyP50, snap.LatencyP99)
	}
}

func TestCollectorActiveGauge(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	c := newCollector([]string{"t1"}, clk, func() float64 { return 0 })

	c.incActive()
	c.incActive()
	c.incActive()
	c.decActive()
	if got := c.snapshot().ActiveConns; got != 2 {
		t.Fatalf("ActiveConns = %d, want 2", got)
	}
	c.decActive()
	c.decActive()
	if got := c.snapshot().ActiveConns; got != 0 {
		t.Fatalf("ActiveConns = %d, want 0", got)
	}
}

func TestCollectorUnknownTargetCountsGloballyOnly(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	c := newCollector([]string{"t1"}, clk, func() float64 { return 0 })

	c.Record(result("t1", protocols.OutcomeSuccess, 1*time.Millisecond, 10, 1))
	c.Record(result("ghost", protocols.OutcomeSuccess, 1*time.Millisecond, 99, 9))

	snap := c.snapshot()
	if snap.Requests != 2 {
		t.Fatalf("Requests = %d, want 2 (global counts both)", snap.Requests)
	}
	if _, ok := snap.PerTarget["ghost"]; ok {
		t.Fatalf("PerTarget must not fabricate a shard for an unknown target")
	}
	if len(snap.PerTarget) != 1 {
		t.Fatalf("PerTarget len = %d, want 1", len(snap.PerTarget))
	}
}
