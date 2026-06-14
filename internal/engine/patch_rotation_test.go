package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// A rotation patch updates the shared rotatingSelector's interval.
func TestApplyPatchRotationInterval(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	newSec := 45
	if err := r.ApplyPatch(context.Background(), ScenarioPatch{RotationIntSec: &newSec}); err != nil {
		t.Fatalf("ApplyPatch rotation: %v", err)
	}
	if got := r.selector.currentInterval(); got != 45*time.Second {
		t.Fatalf("selector interval = %v, want 45s", got)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 1 {
		t.Fatalf("ActionScenarioPatched count = %d, want 1", n)
	}
}

// A negative rotation interval is rejected ClassConfig and leaves the selector alone.
func TestApplyPatchRotationRejectsNegative(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	before := r.selector.currentInterval()
	neg := -1
	err := r.ApplyPatch(context.Background(), ScenarioPatch{RotationIntSec: &neg})
	if err == nil {
		t.Fatal("negative rotation interval must be rejected")
	}
	if !pterr.IsClass(err, pterr.ClassConfig) {
		t.Fatalf("rejection class = %v, want ClassConfig", pterr.Classify(err))
	}
	if got := r.selector.currentInterval(); got != before {
		t.Fatalf("selector interval mutated on rejected patch: %v", got)
	}
}
