package engine

import (
	"context"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/safety"
)

// TargetsDisable force-opens the named target's breaker so the dispatcher skips it
// while siblings keep running. We assert the breaker state flips to BreakerOpen.
// The frozen target IDs for the test scenario are the bare-host strings ("a.example").
func TestApplyPatchTargetsDisableForceOpensBreaker(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	// Frozen target IDs are the bare-host strings validated by scenario.Validate.
	const tid = "a.example"
	if got := r.breakers[tid].State(); got != safety.BreakerClosed {
		t.Fatalf("precondition: breaker for %s = %v, want Closed", tid, got)
	}

	if err := r.ApplyPatch(context.Background(), ScenarioPatch{TargetsDisable: []string{tid}}); err != nil {
		t.Fatalf("ApplyPatch TargetsDisable: %v", err)
	}
	if got := r.breakers[tid].State(); got != safety.BreakerOpen {
		t.Fatalf("breaker for %s = %v, want Open", tid, got)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 1 {
		t.Fatalf("ActionScenarioPatched count = %d, want 1", n)
	}
}

// Disabling an unknown target ID is rejected ClassConfig (no silent no-op).
func TestApplyPatchTargetsDisableUnknownRejected(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	err := r.ApplyPatch(context.Background(), ScenarioPatch{TargetsDisable: []string{"ghost/9"}})
	if err == nil {
		t.Fatal("disabling an unknown target must be rejected")
	}
	if !pterr.IsClass(err, pterr.ClassConfig) {
		t.Fatalf("rejection class = %v, want ClassConfig", pterr.Classify(err))
	}
}
