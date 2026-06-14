package engine

import (
	"context"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// TargetsAdd extends the block's frozen target set through the SAME validate path
// (allowed-domain check) and, on success, appends a stats shard so the new target
// is reportable. We assert the new target ID appears in a fresh snapshot's PerTarget.
func TestApplyPatchTargetsAddExtendsAllowlistAndShard(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	add := []TargetSpec{{BlockID: "web-browsing", Addr: "https://added.example"}}
	if err := r.ApplyPatch(context.Background(), ScenarioPatch{TargetsAdd: add}); err != nil {
		t.Fatalf("ApplyPatch TargetsAdd: %v", err)
	}

	// A shard for the new target must exist in the snapshot (zero counts, but present).
	// The block has 2 existing targets, so the new index is 2.
	snap := r.Snapshot()
	if _, found := snap.PerTarget["web-browsing/2"]; !found {
		t.Fatalf("new target shard missing from snapshot PerTarget: %v", snap.PerTarget)
	}
	if n := sink.count(audit.ActionScenarioPatched); n != 1 {
		t.Fatalf("ActionScenarioPatched count = %d, want 1", n)
	}
}

// An off-allowlist address must be rejected by the SAME Validate path -> ClassConfig,
// and must NOT append a shard.
func TestApplyPatchTargetsAddOffAllowlistRejected(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	add := []TargetSpec{{BlockID: "web-browsing", Addr: "https://evil.example"}}
	err := r.ApplyPatch(context.Background(), ScenarioPatch{TargetsAdd: add})
	if err == nil {
		t.Fatal("off-allowlist target must be rejected")
	}
	if !pterr.IsClass(err, pterr.ClassConfig) {
		t.Fatalf("rejection class = %v, want ClassConfig", pterr.Classify(err))
	}
	snap := r.Snapshot()
	if _, ok := snap.PerTarget["web-browsing/2"]; ok {
		t.Fatal("shard appended for a rejected off-allowlist target")
	}
}

// An unknown block ID is rejected ClassConfig.
func TestApplyPatchTargetsAddUnknownBlockRejected(t *testing.T) {
	sink := &recordingSink{}
	r := newPatchTestRun(t, sink, false)
	defer stopPatchTestRun(t, r)

	add := []TargetSpec{{BlockID: "nope", Addr: "https://a.example"}}
	err := r.ApplyPatch(context.Background(), ScenarioPatch{TargetsAdd: add})
	if err == nil {
		t.Fatal("unknown block must be rejected")
	}
	if !pterr.IsClass(err, pterr.ClassConfig) {
		t.Fatalf("rejection class = %v, want ClassConfig", pterr.Classify(err))
	}
}
