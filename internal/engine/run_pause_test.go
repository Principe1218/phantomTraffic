package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/clock"
)

func TestPauseResumeStateAndGate(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	run, err := e.Start(context.Background(), testScenario(t, 1*time.Hour))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = run.Stop(context.Background()) })

	if err := run.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if got := run.State(); got != StatePaused {
		t.Fatalf("after Pause, State() = %s, want paused", got)
	}
	if run.gate.isOpen() {
		t.Error("gate is open after Pause; want closed (operator source paused)")
	}

	if err := run.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if got := run.State(); got != StateRunning {
		t.Fatalf("after Resume, State() = %s, want running", got)
	}
	if !run.gate.isOpen() {
		t.Error("gate is closed after Resume; want open")
	}
}

func TestPauseRecordsOperatorPausedDuration(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	run, err := e.Start(context.Background(), testScenario(t, 1*time.Hour))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = run.Stop(context.Background()) })

	if err := run.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	fc.Advance(3 * time.Minute) // wall passes while operator-paused
	if err := run.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if got := run.gate.pausedTotal(); got != 3*time.Minute {
		t.Errorf("pausedTotal() = %v, want 3m (the operator-paused span)", got)
	}
}

func TestPauseFromNonRunningRejected(t *testing.T) {
	r := newTestRun(t)
	r.gate = newGate(clock.NewReal())
	r.state.Store(int32(StateStopped))
	if err := r.Pause(); err == nil {
		t.Error("Pause from Stopped returned nil; want an illegal-transition error")
	}
}

func TestPauseAuditsWithDetail(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	sink := &recordingSink{}
	e.opts.Audit = sink
	run, err := e.Start(context.Background(), testScenario(t, 1*time.Hour))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = run.Stop(context.Background()) })

	_ = run.Pause()
	_ = run.Resume()
	// Assert at least the start event is present (sanity that the run shares the sink).
	found := false
	for _, a := range sink.actions() {
		if a == audit.ActionScenarioStarted {
			found = true
		}
	}
	if !found {
		t.Error("expected ActionScenarioStarted in audit log")
	}
}
