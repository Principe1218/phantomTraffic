package safety

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

func testCaps() CapSpec {
	return CapSpec{
		PerTargetRPS:          10,
		GlobalRPS:             50,
		MaxConcurrentSessions: 20,
		TotalRequestBudget:    1_000_000,
		StreamingByteRateKbps: 0,
	}
}

// Acquire returns a usable Reservation when budget is available; Release is
// idempotent and safe to call more than once (always deferred).
func TestLimiterAcquireReleaseIdempotent(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	lim := NewLimiter(clk, testCaps(), NewTripwire(0))

	res, err := lim.Acquire(context.Background(), "t1", 0)
	if err != nil {
		t.Fatalf("Acquire: unexpected error %v", err)
	}
	res.Release()
	res.Release() // must not panic on second call
}

// The per-target bucket is the binding constraint: a single target cannot
// exceed PerTargetRPS even though the global bucket has headroom. Until the
// clock advances, the drained per-target bucket denies, and Acquire surfaces
// a ClassSafety pterr (cap breach) rather than blocking forever.
func TestLimiterPerTargetCapBreachIsClassSafety(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	caps := testCaps()
	caps.PerTargetRPS = 1 // burst 1, no refill without clock advance
	lim := NewLimiter(clk, caps, NewTripwire(0))

	res, err := lim.Acquire(context.Background(), "t1", 0)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	res.Release()

	// Bucket drained, clock frozen => the next Acquire must fail ClassSafety,
	// not block.
	_, err = lim.Acquire(context.Background(), "t1", 0)
	if err == nil {
		t.Fatal("expected ClassSafety error on drained per-target bucket")
	}
	if !pterr.IsClass(err, pterr.ClassSafety) {
		t.Fatalf("got class %v, want ClassSafety", pterr.Classify(err))
	}
}

// A separate target is not penalized by another target's exhaustion: the
// global bucket still has room, so t2 is admitted.
func TestLimiterPerTargetIsolation(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	caps := testCaps()
	caps.PerTargetRPS = 1
	lim := NewLimiter(clk, caps, NewTripwire(0))

	if _, err := lim.Acquire(context.Background(), "t1", 0); err != nil {
		t.Fatalf("t1 Acquire: %v", err)
	}
	// t1 is now drained, but t2 has its own bucket.
	if _, err := lim.Acquire(context.Background(), "t2", 0); err != nil {
		t.Fatalf("t2 should be admitted, got %v", err)
	}
}

// A latched tripwire makes every Acquire fail ClassSafety regardless of budget.
func TestLimiterTrippedTripwireRejects(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	tw := NewTripwire(0)
	tw.Trip("manual")
	lim := NewLimiter(clk, testCaps(), tw)

	_, err := lim.Acquire(context.Background(), "t1", 0)
	if err == nil || !pterr.IsClass(err, pterr.ClassSafety) {
		t.Fatalf("tripped tripwire: got %v, want ClassSafety", err)
	}
}

// A canceled context short-circuits Acquire with the context error.
func TestLimiterAcquireRespectsContext(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	lim := NewLimiter(clk, testCaps(), NewTripwire(0))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lim.Acquire(ctx, "t1", 0); err == nil {
		t.Fatal("expected context error from canceled Acquire")
	}
}

// Reconcile replaces the byte estimate; Release returns reserved byte budget.
// With a streaming byte cap, an over-estimate that is reconciled DOWN frees
// budget so a follow-up small reservation succeeds.
func TestLimiterReservationReconcileFreesBytes(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	caps := testCaps()
	caps.StreamingByteRateKbps = 8 // 8 Kbps = 1000 bytes/sec budget, burst 1000
	lim := NewLimiter(clk, caps, NewTripwire(0))

	// Reserve the whole byte burst with a large estimate.
	res, err := lim.Acquire(context.Background(), "t1", 1000)
	if err != nil {
		t.Fatalf("byte Acquire: %v", err)
	}
	// Actual was tiny; reconcile down then release returns the unused bytes.
	res.Reconcile(10)
	res.Release()

	// Now a fresh small reservation must succeed (budget was returned).
	if _, err := lim.Acquire(context.Background(), "t1", 10); err != nil {
		t.Fatalf("post-reconcile Acquire should succeed, got %v", err)
	}
}

// Saturation is 0 on a fresh limiter that never blocked.
func TestLimiterSaturationStartsZero(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	lim := NewLimiter(clk, testCaps(), NewTripwire(0))
	if s := lim.Saturation(); s != 0 {
		t.Fatalf("fresh Saturation = %v, want 0", s)
	}
}
