package engine

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/rng"
	"github.com/Principe1218/phantomTraffic/internal/safety"
)

// safetyLimiter is a stub Limiter whose Acquire always returns a ClassSafety error,
// simulating a hard cap breach. It lets the worker test the latch path without
// depending on the real token-bucket math.
type safetyLimiter struct{}

func (safetyLimiter) Acquire(_ context.Context, _ string, _ int64) (safety.Reservation, error) {
	return nil, pterr.New(pterr.ClassSafety, "safety.cap", "limiter.Acquire", "hard cap breached")
}
func (safetyLimiter) Saturation() float64 { return 1.0 }

// TestRunActionSafetyTripsTripwireAndStops verifies that a ClassSafety limiter
// error latches the run-level tripwire and stops the worker immediately, recording
// the failing step as a ClassSafety failure and NOT processing later steps.
func TestRunActionSafetyTripsTripwireAndStops(t *testing.T) {
	clk := clock.NewFake(time.Unix(0, 0).UTC())
	r := rng.NewFake(rng.FakeScript{})

	tw := safety.NewTripwire(0)
	h := &fakeHandler{panicOnCall: -1} // must never be reached
	reg := protocols.NewRegistry()
	if err := reg.Register(h); err != nil {
		t.Fatalf("register: %v", err)
	}
	d := workerDeps{
		Clock:             clk,
		Rand:              r,
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		Registry:          reg,
		Limiter:           safetyLimiter{}, // always ClassSafety
		Sem:               newSemaphore(16),
		Stats:             newCollector([]string{"t1"}, clk, func() float64 { return 0 }),
		Gate:              newGate(clk),
		Breakers:          map[string]*safety.Breaker{"t1": safety.NewBreaker(clk, 2, time.Minute)},
		Tripwire:          tw,
		MaxRetries:        3,
		BackoffBase:       10 * time.Millisecond,
		BackoffMax:        time.Second,
		PanicStorm:        4,
		SessionMaxActions: 100,
	}

	tgt := testTarget()
	vs := &fakeVSession{steps: []behavior.Step{
		newStepWithAction(tgt),
		newStepWithAction(tgt), // must never run: worker stopped after the trip
		{Done: true},
	}}
	sess := newTestSession(clk, r)

	done := runWorkerAsync(context.Background(), sess, vs, d)
	time.Sleep(20 * time.Millisecond)
	clk.Advance(time.Second)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runWorker did not return after ClassSafety trip")
	}

	if !tw.Tripped() {
		t.Fatal("tripwire was not tripped by the ClassSafety limiter error")
	}
	if h.callCount() != 0 {
		t.Fatalf("handler Do called %d times, want 0 (limiter rejected before Do)", h.callCount())
	}
	// The worker stopped after the first step; the second action step was never
	// observed.
	if len(vs.observed) != 0 {
		t.Fatalf("session observed %d results, want 0 (worker stops on safety trip before Observe)", len(vs.observed))
	}
}
