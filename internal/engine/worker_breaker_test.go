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
	"github.com/Principe1218/phantomTraffic/internal/safety"
)

// TestRunActionPermanentOpensBreakerThenSkips verifies that N consecutive
// ClassPermanent failures open the per-target breaker (no retries for permanent),
// and a subsequent step against that target is short-circuited as OutcomeSkipped
// so siblings keep running (design §6.5/§7.1).
func TestRunActionPermanentOpensBreakerThenSkips(t *testing.T) {
	clk := clock.NewFake(time.Unix(0, 0).UTC())
	r := rng.NewFake(rng.FakeScript{}) // permanent failures never retry => no jitter draw

	permErr := pterr.New(pterr.ClassPermanent, "noop.gone", "noop.do", "permanent")
	h := &fakeHandler{
		panicOnCall: -1,
		doFn: func(call int) (protocols.Result, protocols.Observation, error) {
			return protocols.Result{Protocol: testProto, Action: "ping", Outcome: protocols.OutcomeFailure, ErrClass: pterr.ClassPermanent}, protocols.Observation{}, permErr
		},
	}

	// Threshold 2: two consecutive permanent failures open the breaker. Cooldown is
	// long so the breaker stays open for the third step (no half-open probe).
	breaker := safety.NewBreaker(clk, 2, time.Hour)
	d := newWorkerDeps(t, clk, r, h, breaker, nil)

	tgt := testTarget()
	// Steps 1+2 fail permanently (open the breaker); step 3 must be skipped.
	vs := &fakeVSession{steps: []behavior.Step{
		newStepWithAction(tgt),
		newStepWithAction(tgt),
		newStepWithAction(tgt),
		{Done: true},
	}}
	sess := newTestSession(clk, r)

	done := runWorkerAsync(context.Background(), sess, vs, d)
	time.Sleep(20 * time.Millisecond)
	clk.Advance(time.Second)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runWorker did not return")
	}

	// The handler is hit only for the two failing steps; the third never reaches it.
	if h.callCount() != 2 {
		t.Fatalf("handler Do called %d times, want 2 (third step skipped by open breaker)", h.callCount())
	}
	if breaker.State() != safety.BreakerOpen {
		t.Fatalf("breaker state = %v, want BreakerOpen after threshold permanent failures", breaker.State())
	}
	if len(vs.observed) != 3 {
		t.Fatalf("session observed %d results, want 3", len(vs.observed))
	}
	if got := vs.observed[2].Outcome; got != protocols.OutcomeSkipped {
		t.Fatalf("third step outcome = %v, want OutcomeSkipped", got)
	}

	snap := d.Stats.snapshot()
	if snap.Failures != 2 {
		t.Fatalf("snapshot Failures = %d, want 2", snap.Failures)
	}
}
