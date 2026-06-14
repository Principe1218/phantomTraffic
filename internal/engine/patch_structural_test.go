package engine

import (
	"context"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// A structural change — a TargetSpec whose Addr names a different protocol
// scheme than the block's protocol — is rejected ClassConfig.
func TestApplyPatchStructuralChangeRejected(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	add := []TargetSpec{{BlockID: "web-browsing", Addr: "ssh://a.example"}}
	err := r.ApplyPatch(context.Background(), ScenarioPatch{TargetsAdd: add})
	if err == nil {
		t.Fatal("a structural (protocol-mismatch) change must be rejected")
	}
	if !pterr.IsClass(err, pterr.ClassConfig) {
		t.Fatalf("rejection class = %v, want ClassConfig", pterr.Classify(err))
	}
	// Rejected patch emits no ActionScenarioPatched.
	if n := sink.count(audit.ActionScenarioPatched); n != 0 {
		t.Fatalf("rejected structural patch emitted ActionScenarioPatched: %d", n)
	}
}

// Every APPLIED patch emits exactly one ActionScenarioPatched, regardless of
// which fields it touched.
func TestApplyPatchAlwaysAuditsAppliedPatch(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	conc := 3
	if err := r.ApplyPatch(context.Background(), ScenarioPatch{Concurrency: &conc}); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	rot := 10
	if err := r.ApplyPatch(context.Background(), ScenarioPatch{RotationIntSec: &rot}); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 2 {
		t.Fatalf("ActionScenarioPatched count = %d, want 2 (one per applied patch)", n)
	}
}

// An entirely empty patch is still an applied patch and is audited once.
func TestApplyPatchEmptyIsAudited(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	if err := r.ApplyPatch(context.Background(), ScenarioPatch{}); err != nil {
		t.Fatalf("empty ApplyPatch: %v", err)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 1 {
		t.Fatalf("ActionScenarioPatched count = %d, want 1 for an empty patch", n)
	}
}
