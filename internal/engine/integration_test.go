package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
)

// TestEngineIntegration_GoldenRun verifies the happy path: a run starts,
// workers execute actions through the noop handler, and the run reaches
// StateCompleted once the block's duration elapses on the fake clock.
func TestEngineIntegration_GoldenRun(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	sc := testScenario(t, time.Minute)

	run, err := e.Start(context.Background(), sc)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = run.Stop(context.Background()) })

	// Let the supervisor goroutines (runBlockDuration, workers) start and register
	// their timers before advancing the fake clock past the block duration.
	time.Sleep(20 * time.Millisecond)
	fc.Advance(time.Minute)

	select {
	case <-run.Wait():
	case <-time.After(2 * time.Second):
		t.Fatalf("run did not complete within timeout; State=%s", run.State())
	}

	if got := run.State(); got != StateCompleted {
		t.Errorf("State() = %s, want completed", got)
	}
	if err := run.Err(); err != nil {
		t.Errorf("Err() = %v, want nil after clean completion", err)
	}
	snap := run.Snapshot()
	if snap.Requests == 0 {
		t.Error("snapshot shows 0 requests; want > 0 (workers should have executed actions)")
	}
}

// TestEngineIntegration_PanicIsolation verifies that a handler that panics on
// every invocation does not crash or deadlock the engine. Workers recover from
// panics, record OutcomePanicked, and drain cleanly on Stop.
func TestEngineIntegration_PanicIsolation(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())

	reg := protocols.NewRegistry()
	if err := reg.Register(NoopHandler{Panic: true}); err != nil {
		t.Fatalf("register panicking noop: %v", err)
	}
	opts := validOptions()
	opts.Clock = fc
	opts.Registry = reg
	e, err := New(opts)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}

	sc := testScenario(t, 10*time.Minute)
	run, err := e.Start(context.Background(), sc)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Advance past the first think-time samples so workers reach handler.Do and
	// panic. The engine must survive (runGuarded catches every panic).
	time.Sleep(20 * time.Millisecond)
	fc.Advance(5 * time.Second)
	time.Sleep(20 * time.Millisecond)

	// Stop must drain cleanly even with workers that keep panicking.
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := run.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v — engine may have leaked goroutines or timed out", err)
	}
	if got := run.State(); got != StateStopped {
		t.Errorf("State() = %s, want stopped", got)
	}
}

// TestEngineIntegration_PauseResumeTimerShift verifies the gate-aware block
// duration timer: advancing the fake clock while the run is paused must NOT
// count toward the block's active duration and must NOT trigger early completion.
// Only active (un-paused) time counts.
func TestEngineIntegration_PauseResumeTimerShift(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	sc := testScenario(t, time.Minute) // 60 s active duration

	run, err := e.Start(context.Background(), sc)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = run.Stop(context.Background()) })

	// Let runBlockDuration register its 60 s timer before we advance.
	time.Sleep(20 * time.Millisecond)

	// Advance 30 s (half duration). Run must remain active.
	fc.Advance(30 * time.Second)

	// Pause: runBlockDuration's closeNotify fires, it records 30 s elapsed and
	// blocks in waitOpenAndGetCloseCh until the gate reopens.
	if err := run.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	time.Sleep(20 * time.Millisecond) // let runBlockDuration record elapsed and park

	// Advance 5 min while paused. The run must NOT complete.
	fc.Advance(5 * time.Minute)
	time.Sleep(10 * time.Millisecond)
	select {
	case <-run.Wait():
		t.Fatal("run completed while paused — timer shift failed")
	default:
	}

	// Resume: runBlockDuration unblocks and creates a new 30 s timer for the
	// remaining active duration.
	if err := run.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	time.Sleep(20 * time.Millisecond) // let runBlockDuration register the new 30 s timer

	// Advance the remaining 30 s — the run must complete now.
	fc.Advance(30 * time.Second)

	select {
	case <-run.Wait():
	case <-time.After(2 * time.Second):
		t.Fatalf("run did not complete after remaining 30 s active time; State=%s", run.State())
	}

	if got := run.State(); got != StateCompleted {
		t.Errorf("State() = %s, want completed", got)
	}
	if err := run.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}
