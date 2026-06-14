package engine

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/persona"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/rng"
	"github.com/Principe1218/phantomTraffic/internal/safety"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

func f64(v float64) *float64 { return &v }

func TestApplyPatchCapsDownFree(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false /* capOverride */)
	defer stopPatchTestRun(t, r)

	// Existing limit is GlobalRPS=50 (from the validated test scenario). Lower it.
	before := r.caps.GlobalRPS
	lower := before - 10
	if err := r.ApplyPatch(context.Background(), ScenarioPatch{
		Caps: &safety.CapPatch{GlobalRPS: f64(lower)},
	}); err != nil {
		t.Fatalf("lowering a cap must be free, got: %v", err)
	}
	if r.caps.GlobalRPS != lower {
		t.Fatalf("caps.GlobalRPS = %v, want %v", r.caps.GlobalRPS, lower)
	}
	// Lowering does NOT emit a cap-override audit event.
	if n := sink.count(audit.ActionCapOverrideEnabled); n != 0 {
		t.Fatalf("cap-override audited on a DOWN patch: %d", n)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 1 {
		t.Fatalf("ActionScenarioPatched count = %d, want 1", n)
	}
}

func TestApplyPatchCapsUpRejectedWithoutOverride(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false /* capOverride */)
	defer stopPatchTestRun(t, r)

	before := r.caps.GlobalRPS
	err := r.ApplyPatch(context.Background(), ScenarioPatch{
		Caps: &safety.CapPatch{GlobalRPS: f64(before + 100)},
	})
	if err == nil {
		t.Fatal("raising a cap without override must be rejected")
	}
	if !pterr.IsClass(err, pterr.ClassConfig) {
		t.Fatalf("cap-up rejection class = %v, want ClassConfig", pterr.Classify(err))
	}
	if r.caps.GlobalRPS != before {
		t.Fatalf("caps mutated on a rejected UP patch: %v", r.caps.GlobalRPS)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 0 {
		t.Fatalf("rejected patch must not emit ActionScenarioPatched: %d", n)
	}
}

func TestApplyPatchCapsUpWithOverrideAudited(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, true /* capOverride */)
	defer stopPatchTestRun(t, r)

	before := r.caps.GlobalRPS
	raise := before + 100
	if err := r.ApplyPatch(context.Background(), ScenarioPatch{
		Caps: &safety.CapPatch{GlobalRPS: f64(raise)},
	}); err != nil {
		t.Fatalf("raising a cap under override must succeed, got: %v", err)
	}
	if r.caps.GlobalRPS != raise {
		t.Fatalf("caps.GlobalRPS = %v, want %v", r.caps.GlobalRPS, raise)
	}
	if n := sink.count(audit.ActionCapOverrideEnabled); n != 1 {
		t.Fatalf("ActionCapOverrideEnabled count = %d, want 1", n)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 1 {
		t.Fatalf("ActionScenarioPatched count = %d, want 1", n)
	}
}

// ----- shared helpers used by E6.2–E6.8 -----

func newPatchTestRun(t *testing.T, sink audit.Sink, capOverride bool) *Run {
	t.Helper()
	clk := clock.NewFake(time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC))
	rnd := rng.NewFake(rng.FakeScript{
		Floats: make([]float64, 4096),
		Norms:  make([]float64, 4096),
		Exps:   make([]float64, 4096),
		Ints:   make([]int, 4096),
	})
	reg := protocols.NewRegistry()
	if err := reg.Register(NoopHandler{}); err != nil {
		t.Fatalf("register noop: %v", err)
	}
	eng, err := New(Options{
		Clock: clk, Rand: rnd, Registry: reg, SessionMaker: behavior.NewSessionMaker(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Audit: sink, AgentID: "agent-test",
		StatsInterval: time.Second, GraceTimeout: time.Second,
		MaxRetries: 2, BackoffBase: time.Millisecond, BackoffMax: time.Second,
	})
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	sc := patchTestScenario(t, capOverride)
	run, err := eng.Start(context.Background(), sc)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return run
}

func stopPatchTestRun(t *testing.T, r *Run) {
	t.Helper()
	_ = r.Stop(context.Background())
}

// patchTestScenario builds a validated single-block HTTP scenario routed to NoopHandler.
// Targets are bare hosts (no scheme) as required by scenario.Validate.
func patchTestScenario(t *testing.T, capOverride bool) scenario.Scenario {
	t.Helper()
	raw := scenario.Raw{
		Name:           "patch-test",
		AllowedDomains: []string{"a.example", "b.example", "added.example"},
		Scenarios: []scenario.RawBlock{{
			ID:              "web-browsing",
			Protocol:        "http",
			Targets:         []string{"a.example", "b.example"},
			Persona:         persona.DefaultPersonaName,
			Concurrency:     4,
			DurationMinutes: 30,
			Weight:          100,
		}},
	}
	sc, err := scenario.Validate(raw, scenario.Options{CapOverride: capOverride, AgentCount: 1})
	if err != nil {
		t.Fatalf("validate test scenario: %v", err)
	}
	return sc
}
