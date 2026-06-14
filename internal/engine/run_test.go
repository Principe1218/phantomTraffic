package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/persona"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/safety"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// testScenario builds a single-block scenario routed to the noop handler under
// ProtoHTTP, with a short duration so the AfterFunc completion path is easy to drive.
func testScenario(t *testing.T, dur time.Duration) scenario.Scenario {
	t.Helper()
	builtins, err := persona.Builtins()
	if err != nil {
		t.Fatalf("persona.Builtins: %v", err)
	}
	p := builtins[persona.DefaultPersonaName]

	tgt := protocols.Target{ID: "t1", Proto: protocols.ProtoHTTP, Addr: "noop.example"}
	ts := protocols.NewTargetSet([]protocols.Target{tgt}, []string{"noop.example"})

	return scenario.Scenario{
		Name:           "lifecycle-test",
		AllowedDomains: []string{"noop.example"},
		AgentCount:     1,
		Caps:           safety.CapSpec{PerTargetRPS: 10, GlobalRPS: 50, MaxConcurrentSessions: 5, TotalRequestBudget: 1_000_000},
		Ceiling:        safety.DefaultCeiling(),
		Execution:      scenario.Execution{Mode: scenario.ExecParallel},
		Blocks: []scenario.Block{{
			ID:          "web",
			Protocol:    protocols.ProtoHTTP,
			Targets:     []protocols.Target{tgt},
			Rotation:    scenario.RotationSequential,
			Persona:     p,
			Concurrency: 2,
			Duration:    dur,
			Weight:      1,
		}},
		Targets: ts,
	}
}

// newEngineWithNoop builds an Engine on a fake clock with the noop handler
// registered under ProtoHTTP.
func newEngineWithNoop(t *testing.T, fc *clock.FakeClock) *Engine {
	t.Helper()
	reg := protocols.NewRegistry()
	if err := reg.Register(NoopHandler{}); err != nil {
		t.Fatalf("register noop: %v", err)
	}
	opts := validOptions()
	opts.Clock = fc
	opts.Registry = reg
	opts.StatsInterval = time.Second
	e, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

func TestStartReachesRunning(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	sc := testScenario(t, 10*time.Minute)

	run, err := e.Start(context.Background(), sc)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = run.Stop(context.Background()) })

	if got := run.State(); got != StateRunning {
		t.Fatalf("after Start, State() = %s, want running", got)
	}
	if run.ID() == "" {
		t.Error("run.ID() empty; want a non-empty correlation id")
	}
	if run.AgentID() != "agent-test" {
		t.Errorf("run.AgentID() = %q, want agent-test", run.AgentID())
	}
}

func TestStartCompletesWhenDurationElapses(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	sc := testScenario(t, 5*time.Minute)

	run, err := e.Start(context.Background(), sc)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = run.Stop(context.Background()) })

	// Let the supervisor goroutines (including the AfterFunc completion timer)
	// register before advancing the fake clock past the block duration.
	time.Sleep(20 * time.Millisecond)
	fc.Advance(5 * time.Minute)

	select {
	case <-run.Wait():
	case <-time.After(2 * time.Second):
		t.Fatalf("run did not complete within timeout; State() = %s", run.State())
	}

	if got := run.State(); got != StateCompleted {
		t.Errorf("after duration elapse, State() = %s, want completed", got)
	}
	if err := run.Err(); err != nil {
		t.Errorf("Err() = %v after clean completion, want nil", err)
	}
}

// satisfy the imports used only by later tasks editing this file
var _ = behavior.NewSessionMaker
