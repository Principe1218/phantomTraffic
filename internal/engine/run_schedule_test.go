package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// scenarioWithSchedule builds a single-block scenario whose schedule has one
// window 08:00-18:00 UTC on every day, so an off-window edge is reachable by
// advancing the fake clock past 18:00.
func scenarioWithSchedule(t *testing.T, start time.Time) scenario.Scenario {
	t.Helper()
	sc := testScenario(t, 24*time.Hour)
	var allDays [7]bool
	for i := range allDays {
		allDays[i] = true
	}
	sc.Schedule = scenario.Schedule{
		Loc: time.UTC,
		Windows: []scenario.ScheduleWindow{{
			Days:  allDays,
			Start: 8 * time.Hour,
			End:   18 * time.Hour,
		}},
	}
	return sc
}

func TestSchedulerPausesOnOffWindowEdge(t *testing.T) {
	// Start at 17:00 UTC: inside the window, run is Running and the schedule
	// gate source is open.
	start := time.Date(2026, 6, 15, 17, 0, 0, 0, time.UTC)
	fc := clock.NewFake(start)
	e := newEngineWithNoop(t, fc)

	run, err := e.Start(context.Background(), scenarioWithSchedule(t, start))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = run.Stop(context.Background()) })

	if !run.gate.isOpen() {
		t.Fatal("gate closed at run start inside the active window; want open")
	}

	// Let the scheduler goroutine register its timer (sleeps to NextTransition),
	// then advance past 18:00 to cross the off-window edge.
	time.Sleep(20 * time.Millisecond)
	fc.Advance(time.Hour + time.Minute) // -> 18:01, now outside the window

	// The scheduler should have paused the schedule gate source. Poll briefly to
	// avoid a race with the goroutine waking from the fake timer.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !run.gate.isOpen() {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if run.gate.isOpen() {
		t.Error("gate still open after crossing the off-window edge; want schedule source paused")
	}
	// Operator state stays Running; only the schedule gate source is paused.
	if got := run.State(); got != StateRunning {
		t.Errorf("State() = %s after schedule pause, want running (schedule is a gate source, not a state)", got)
	}
}

func TestNoSchedulerWhenNoWindows(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	run, err := e.Start(context.Background(), testScenario(t, time.Hour)) // empty Schedule
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = run.Stop(context.Background()) })
	// No windows -> always active -> gate open and no schedule pause ever.
	if !run.gate.isOpen() {
		t.Error("gate closed with no schedule windows; want always open")
	}
}
