package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// TestRunActionTransientRetryThenSuccess verifies a ClassTransient failure on the
// first attempt is retried (after a clock-driven backoff) and the second attempt's
// success is the recorded outcome. The fake rng scripts the single Float64 jitter
// draw; the fake clock must be advanced past the backoff delay to unblock the
// retry sleep (register-before-advance idiom).
func TestRunActionTransientRetryThenSuccess(t *testing.T) {
	clk := clock.NewFake(time.Unix(0, 0).UTC())
	// One backoff between attempt 0 and attempt 1: attempt 0, base 10ms, jitter 1.0
	// => exp = 10ms, delay = 10ms.
	r := rng.NewFake(rng.FakeScript{Floats: []float64{1.0}})

	transientErr := pterr.New(pterr.ClassTransient, "noop.flaky", "noop.do", "temporary")
	h := &fakeHandler{
		panicOnCall: -1,
		doFn: func(call int) (protocols.Result, protocols.Observation, error) {
			if call == 0 {
				return protocols.Result{Protocol: testProto, Action: "ping", Outcome: protocols.OutcomeFailure, ErrClass: pterr.ClassTransient}, protocols.Observation{}, transientErr
			}
			return protocols.Result{Protocol: testProto, Action: "ping", Outcome: protocols.OutcomeSuccess}, protocols.Observation{}, nil
		},
	}

	d := newWorkerDeps(t, clk, r, h, nil, nil)
	tgt := testTarget()
	vs := &fakeVSession{steps: []behavior.Step{newStepWithAction(tgt), {Done: true}}}
	sess := newTestSession(clk, r)

	done := runWorkerAsync(context.Background(), sess, vs, d, clk)

	// Let the worker reach the retry backoff Sleep, then advance past it.
	time.Sleep(20 * time.Millisecond)
	clk.Advance(50 * time.Millisecond)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runWorker did not return; retry path may be stuck")
	}

	if h.callCount() != 2 {
		t.Fatalf("handler Do called %d times, want 2 (1 transient + 1 success)", h.callCount())
	}
	if len(vs.observed) != 1 {
		t.Fatalf("session observed %d results, want 1", len(vs.observed))
	}
	if got := vs.observed[0].Outcome; got != protocols.OutcomeSuccess {
		t.Fatalf("final outcome = %v, want OutcomeSuccess after retry", got)
	}

	snap := d.Stats.snapshot()
	if snap.Successes != 1 || snap.Failures != 0 {
		t.Fatalf("snapshot Successes=%d Failures=%d, want 1/0 (the retried-then-succeeded step counts once, as success)", snap.Successes, snap.Failures)
	}
}
