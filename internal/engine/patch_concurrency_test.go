package engine

import (
	"context"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// A concurrency patch must resize the live admission semaphore via setLimit,
// not mutate any other surface. We observe the semaphore's effective limit.
func TestApplyPatchConcurrencyResizesSemaphore(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	newLimit := 2
	if err := r.ApplyPatch(context.Background(), ScenarioPatch{Concurrency: &newLimit}); err != nil {
		t.Fatalf("ApplyPatch concurrency: %v", err)
	}
	if got := r.sem.currentLimit(); got != newLimit {
		t.Fatalf("semaphore limit = %d, want %d", got, newLimit)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 1 {
		t.Fatalf("ActionScenarioPatched count = %d, want 1", n)
	}
}

// Concurrency must be bounded: a non-positive value is rejected ClassConfig and
// leaves the semaphore untouched.
func TestApplyPatchConcurrencyRejectsNonPositive(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	before := r.sem.currentLimit()
	zero := 0
	err := r.ApplyPatch(context.Background(), ScenarioPatch{Concurrency: &zero})
	if err == nil {
		t.Fatal("non-positive concurrency must be rejected")
	}
	if !pterr.IsClass(err, pterr.ClassConfig) {
		t.Fatalf("rejection class = %v, want ClassConfig", pterr.Classify(err))
	}
	if got := r.sem.currentLimit(); got != before {
		t.Fatalf("semaphore mutated on rejected patch: %d (was %d)", got, before)
	}
}
