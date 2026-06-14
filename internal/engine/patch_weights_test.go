package engine

import (
	"context"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// A weights patch stores the new, re-normalized weights on the run so that the
// next vuser recycle picks them up. We assert the stored map is GCD-reduced and
// that the patch is audited.
func TestApplyPatchWeightsRenormalize(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	w := MixWeights{"web-browsing": 200} // single block; normalizes to 1
	if err := r.ApplyPatch(context.Background(), ScenarioPatch{Weights: &w}); err != nil {
		t.Fatalf("ApplyPatch weights: %v", err)
	}
	if got := r.weights["web-browsing"]; got != 1 {
		t.Fatalf("normalized weight = %d, want 1", got)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 1 {
		t.Fatalf("ActionScenarioPatched count = %d, want 1", n)
	}
}

// A weight referencing an unknown block ID is a structural mismatch -> ClassConfig.
func TestApplyPatchWeightsUnknownBlockRejected(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	w := MixWeights{"does-not-exist": 5}
	err := r.ApplyPatch(context.Background(), ScenarioPatch{Weights: &w})
	if err == nil {
		t.Fatal("weight on unknown block must be rejected")
	}
	if !pterr.IsClass(err, pterr.ClassConfig) {
		t.Fatalf("rejection class = %v, want ClassConfig", pterr.Classify(err))
	}
}

// A zero weight is invalid (every block must keep positive weight) -> ClassConfig.
func TestApplyPatchWeightsZeroRejected(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	w := MixWeights{"web-browsing": 0}
	err := r.ApplyPatch(context.Background(), ScenarioPatch{Weights: &w})
	if err == nil {
		t.Fatal("zero weight must be rejected")
	}
	if !pterr.IsClass(err, pterr.ClassConfig) {
		t.Fatalf("rejection class = %v, want ClassConfig", pterr.Classify(err))
	}
}
