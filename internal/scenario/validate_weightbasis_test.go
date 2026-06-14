package scenario

import (
    "path/filepath"
    "testing"
)

func TestValidateWeightBasisDefaults(t *testing.T) {
    raw := baseRaw()
    raw.Scenarios[0].Concurrency = 1
    raw.Scenarios[0].DurationMinutes = 5
    raw.WeightBasis = "" // omitted
    sc, err := Validate(raw, Options{AgentCount: 1})
    if err != nil {
        t.Fatalf("Validate returned error: %v", err)
    }
    assertEqual(t, "defaulted Scenario.WeightBasis", sc.WeightBasis, WeightByVuserPopulation)
}

func TestValidateWeightBasisParsed(t *testing.T) {
    tests := []struct {
        in   string
        want WeightBasis
    }{
        {"vuser_population", WeightByVuserPopulation},
        {"concurrency", WeightByConcurrency},
        {"request_rate", WeightByRequestRate},
    }
    for _, tt := range tests {
        t.Run(tt.in, func(t *testing.T) {
            raw := baseRaw()
            raw.Scenarios[0].Concurrency = 1
            raw.Scenarios[0].DurationMinutes = 5
            raw.WeightBasis = tt.in
            sc, err := Validate(raw, Options{AgentCount: 1})
            if err != nil {
                t.Fatalf("Validate returned error: %v", err)
            }
            assertEqual(t, "Scenario.WeightBasis", sc.WeightBasis, tt.want)
        })
    }
}

func TestValidateWeightBasisRejectsUnknown(t *testing.T) {
    raw := baseRaw()
    raw.Scenarios[0].Concurrency = 1
    raw.Scenarios[0].DurationMinutes = 5
    raw.WeightBasis = "bytes"
    _, err := Validate(raw, Options{AgentCount: 1})
    assertFieldError(t, requireValidationErrors(t, err), "weight_basis", "unknown weight_basis")
}

func TestValidateFreezesEngineFieldsFromFixture(t *testing.T) {
    // Full Load -> Validate round-trip of the engine_fields fixture (Task S3).
    raw, err := Load(filepath.Join("testdata", "engine_fields.yaml"))
    if err != nil {
        t.Fatalf("Load(engine_fields.yaml): %v", err)
    }
    sc, err := Validate(raw, Options{AgentCount: 1})
    if err != nil {
        t.Fatalf("Validate(engine_fields): %v", err)
    }
    assertEqual(t, "frozen Scenario.WeightBasis", sc.WeightBasis, WeightByConcurrency)
    assertEqual(t, "frozen Schedule.Loc.String()", sc.Schedule.Loc.String(), "America/New_York")
    assertEqual(t, "frozen len(Schedule.Windows)", len(sc.Schedule.Windows), 1)
    assertEqual(t, "frozen Block.Concurrency", sc.Blocks[0].Concurrency, 5)
    assertEqual(t, "frozen Block.Ramp.Up", sc.Blocks[0].Ramp.StartConcurrency, 1)
}
