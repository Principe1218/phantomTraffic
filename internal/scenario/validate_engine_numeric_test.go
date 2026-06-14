package scenario

import (
	"testing"
	"time"
)

func TestValidateConcurrencyDefaultsAndBuilds(t *testing.T) {
	raw := baseRaw()
	raw.Scenarios[0].Concurrency = 5
	raw.Scenarios[0].DurationMinutes = 30
	raw.Scenarios[0].Weight = 70
	sc, err := Validate(raw, Options{AgentCount: 1})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	assertEqual(t, "Block.Concurrency", sc.Blocks[0].Concurrency, 5)
	assertEqual(t, "Block.Duration", sc.Blocks[0].Duration, 30*time.Minute)
	assertEqual(t, "Block.Weight", sc.Blocks[0].Weight, uint(70))
}

func TestValidateConcurrencyAndWeightDefaultToOne(t *testing.T) {
	// Omitted concurrency (0) defaults to 1; omitted weight (0) defaults to 1.
	raw := baseRaw()
	raw.Scenarios[0].Concurrency = 0
	raw.Scenarios[0].DurationMinutes = 10
	raw.Scenarios[0].Weight = 0
	sc, err := Validate(raw, Options{AgentCount: 1})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	assertEqual(t, "defaulted Block.Concurrency", sc.Blocks[0].Concurrency, 1)
	assertEqual(t, "defaulted Block.Weight", sc.Blocks[0].Weight, uint(1))
}

func TestValidateDurationRequired(t *testing.T) {
	raw := baseRaw()
	raw.Scenarios[0].Concurrency = 1
	raw.Scenarios[0].DurationMinutes = 0 // missing -> error
	_, err := Validate(raw, Options{AgentCount: 1})
	assertFieldError(t, requireValidationErrors(t, err), "scenarios[0].duration_minutes", ">= 1")
}

func TestValidateConcurrencyUpperBoundAgainstEffectiveCap(t *testing.T) {
	// baseRaw has no caps block, so at AgentCount 1 the effective
	// MaxConcurrentSessions inherits the ceiling (20). 20 is valid, 21 is rejected.
	valid := baseRaw()
	valid.Scenarios[0].Concurrency = 20
	valid.Scenarios[0].DurationMinutes = 5
	if _, err := Validate(valid, Options{AgentCount: 1}); err != nil {
		t.Fatalf("concurrency 20 should be valid at the default ceiling, got: %v", err)
	}

	over := baseRaw()
	over.Scenarios[0].Concurrency = 21
	over.Scenarios[0].DurationMinutes = 5
	_, err := Validate(over, Options{AgentCount: 1})
	assertFieldError(t, requireValidationErrors(t, err), "scenarios[0].concurrency", "exceeds")
}

func TestValidateConcurrencyBoundFollowsDeclaredCap(t *testing.T) {
	// A declared max_concurrent_sessions tightens the bound below the ceiling.
	raw := baseRaw()
	raw.Caps = RawCaps{MaxConcurrentSessions: 4}
	raw.Scenarios[0].Concurrency = 5 // > 4 declared cap
	raw.Scenarios[0].DurationMinutes = 5
	_, err := Validate(raw, Options{AgentCount: 1})
	assertFieldError(t, requireValidationErrors(t, err), "scenarios[0].concurrency", "exceeds")
}
