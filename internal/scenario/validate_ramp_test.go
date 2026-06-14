package scenario

import (
	"testing"
	"time"
)

// rampRaw returns a baseRaw with a single block carrying the given load fields,
// ready for the caller to set or clear Scenarios[0].Ramp.
func rampRaw(concurrency, durationMinutes int) Raw {
	raw := baseRaw()
	raw.Scenarios[0].Concurrency = concurrency
	raw.Scenarios[0].DurationMinutes = durationMinutes
	raw.Scenarios[0].Weight = 1
	return raw
}

func TestValidateNilRampBuildsZeroPlan(t *testing.T) {
	raw := rampRaw(5, 10)
	raw.Scenarios[0].Ramp = nil
	sc, err := Validate(raw, Options{AgentCount: 1})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	assertEqual(t, "no-ramp Block.Ramp.Up", sc.Blocks[0].Ramp.Up, time.Duration(0))
	assertEqual(t, "no-ramp Block.Ramp.StartConcurrency", sc.Blocks[0].Ramp.StartConcurrency, 0)
}

func TestValidateValidRampBuilds(t *testing.T) {
	raw := rampRaw(5, 10)
	raw.Scenarios[0].Ramp = &RawRamp{UpSeconds: 60, StartConcurrency: 1}
	sc, err := Validate(raw, Options{AgentCount: 1})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	assertEqual(t, "Block.Ramp.Up", sc.Blocks[0].Ramp.Up, 60*time.Second)
	assertEqual(t, "Block.Ramp.StartConcurrency", sc.Blocks[0].Ramp.StartConcurrency, 1)
}

func TestValidateRampStartConcurrencyDefaultsToConcurrency(t *testing.T) {
	// start_concurrency 0 defaults to the block concurrency => effectively no ramp.
	raw := rampRaw(5, 10)
	raw.Scenarios[0].Ramp = &RawRamp{UpSeconds: 30, StartConcurrency: 0}
	sc, err := Validate(raw, Options{AgentCount: 1})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	assertEqual(t, "defaulted Ramp.StartConcurrency", sc.Blocks[0].Ramp.StartConcurrency, 5)
}

func TestValidateRampErrors(t *testing.T) {
	tests := []struct {
		name      string
		ramp      RawRamp
		wantField string
		wantSub   string
	}{
		{
			name:      "negative up_seconds",
			ramp:      RawRamp{UpSeconds: -1, StartConcurrency: 1},
			wantField: "scenarios[0].ramp.up_seconds",
			wantSub:   ">= 0",
		},
		{
			name:      "up_seconds exceeds duration",
			ramp:      RawRamp{UpSeconds: 601, StartConcurrency: 1}, // duration 10m = 600s
			wantField: "scenarios[0].ramp.up_seconds",
			wantSub:   "duration",
		},
		{
			name:      "start_concurrency above concurrency",
			ramp:      RawRamp{UpSeconds: 30, StartConcurrency: 6}, // concurrency 5
			wantField: "scenarios[0].ramp.start_concurrency",
			wantSub:   "1..",
		},
		{
			name:      "start_concurrency below 1",
			ramp:      RawRamp{UpSeconds: 30, StartConcurrency: -1},
			wantField: "scenarios[0].ramp.start_concurrency",
			wantSub:   "1..",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := rampRaw(5, 10)
			ramp := tt.ramp
			raw.Scenarios[0].Ramp = &ramp
			_, err := Validate(raw, Options{AgentCount: 1})
			assertFieldError(t, requireValidationErrors(t, err), tt.wantField, tt.wantSub)
		})
	}
}
