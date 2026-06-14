package engine

import (
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/safety"
)

// Tests that the pinned patch types exist with the pinned field shapes and that a
// ScenarioPatch can be assembled from every field. Pure value test, no Run needed.
func TestScenarioPatchTypesAssemble(t *testing.T) {
	perTarget := 4.0
	conc := 8
	rot := 30
	weights := MixWeights{"web-browsing": 70, "api-poll": 30}
	p := ScenarioPatch{
		Caps:           &safety.CapPatch{PerTargetRPS: &perTarget},
		Concurrency:    &conc,
		Weights:        &weights,
		RotationIntSec: &rot,
		TargetsAdd:     []TargetSpec{{BlockID: "web-browsing", Addr: "https://added.example"}},
		TargetsDisable: []string{"web-browsing/0"},
	}

	if p.Caps == nil || p.Caps.PerTargetRPS == nil || *p.Caps.PerTargetRPS != 4.0 {
		t.Fatalf("Caps not wired through: %+v", p.Caps)
	}
	if p.Concurrency == nil || *p.Concurrency != 8 {
		t.Fatalf("Concurrency = %v, want 8", p.Concurrency)
	}
	if p.RotationIntSec == nil || *p.RotationIntSec != 30 {
		t.Fatalf("RotationIntSec = %v, want 30", p.RotationIntSec)
	}
	if got := (*p.Weights)["web-browsing"]; got != 70 {
		t.Fatalf("Weights[web-browsing] = %d, want 70", got)
	}
	if len(p.TargetsAdd) != 1 || p.TargetsAdd[0].BlockID != "web-browsing" {
		t.Fatalf("TargetsAdd not wired: %+v", p.TargetsAdd)
	}
	if p.TargetsAdd[0].Addr != "https://added.example" {
		t.Fatalf("TargetSpec.Addr = %q", p.TargetsAdd[0].Addr)
	}
	if len(p.TargetsDisable) != 1 || p.TargetsDisable[0] != "web-browsing/0" {
		t.Fatalf("TargetsDisable not wired: %+v", p.TargetsDisable)
	}
}

// An empty patch must be a valid zero value (every field nil/empty).
func TestScenarioPatchZeroValue(t *testing.T) {
	var p ScenarioPatch
	if p.Caps != nil || p.Concurrency != nil || p.Weights != nil ||
		p.RotationIntSec != nil || p.TargetsAdd != nil || p.TargetsDisable != nil {
		t.Fatalf("zero ScenarioPatch is not empty: %+v", p)
	}
}
