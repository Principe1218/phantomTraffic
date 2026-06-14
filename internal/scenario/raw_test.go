package scenario

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

func TestLoadDecodesEngineFields(t *testing.T) {
	raw, err := Load(filepath.Join("testdata", "engine_fields.yaml"))
	if err != nil {
		t.Fatalf("Load(engine_fields.yaml) returned error: %v", err)
	}

	// Scenario-level additions.
	expectEqual(t, "Raw.WeightBasis", raw.WeightBasis, "concurrency")
	if raw.Schedule == nil {
		t.Fatal("Raw.Schedule decoded as nil, want a populated *RawSchedule")
	}
	expectEqual(t, "Schedule.Timezone", raw.Schedule.Timezone, "America/New_York")
	assertEqual(t, "len(Schedule.Windows)", len(raw.Schedule.Windows), 1)
	w := raw.Schedule.Windows[0]
	assertEqual(t, "len(Schedule.Windows[0].Days)", len(w.Days), 5)
	expectEqual(t, "Schedule.Windows[0].Days[0]", w.Days[0], "mon")
	expectEqual(t, "Schedule.Windows[0].Start", w.Start, "08:00")
	expectEqual(t, "Schedule.Windows[0].End", w.End, "18:00")

	// Block-level additions.
	assertEqual(t, "len(Scenarios)", len(raw.Scenarios), 1)
	b := raw.Scenarios[0]
	expectEqual(t, "Scenarios[0].Concurrency", b.Concurrency, 5)
	expectEqual(t, "Scenarios[0].DurationMinutes", b.DurationMinutes, 30)
	expectEqual(t, "Scenarios[0].Weight", b.Weight, uint(70))
	if b.Ramp == nil {
		t.Fatal("Scenarios[0].Ramp decoded as nil, want a populated *RawRamp")
	}
	expectEqual(t, "Scenarios[0].Ramp.UpSeconds", b.Ramp.UpSeconds, 60)
	expectEqual(t, "Scenarios[0].Ramp.StartConcurrency", b.Ramp.StartConcurrency, 1)
}

func TestLoadOmittedEngineFieldsLeaveNilPointers(t *testing.T) {
	// The Plan-2 valid.yaml fixture has no ramp/schedule; the optional pointers
	// must decode as nil so Validate can treat them as "unset".
	raw, err := Load(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatalf("Load(valid.yaml) returned error: %v", err)
	}
	if raw.Schedule != nil {
		t.Fatalf("Raw.Schedule = %+v, want nil when the key is omitted", raw.Schedule)
	}
	if raw.Scenarios[0].Ramp != nil {
		t.Fatalf("Scenarios[0].Ramp = %+v, want nil when the key is omitted", raw.Scenarios[0].Ramp)
	}
	expectEqual(t, "omitted Raw.WeightBasis", raw.WeightBasis, "")
	expectEqual(t, "omitted Scenarios[0].Concurrency", raw.Scenarios[0].Concurrency, 0)
}

func TestLoadRejectsUnknownEngineKey(t *testing.T) {
	// Strict decode must still reject a typo'd key under the new ramp surface.
	_, err := Load(filepath.Join("testdata", "engine_unknown_key.yaml"))
	if err == nil {
		t.Fatal("Load(engine_unknown_key.yaml) = nil error, want a strict-decode failure")
	}
	var pe *pterr.Error
	if !errors.As(err, &pe) {
		t.Fatalf("error is not a *pterr.Error: %v", err)
	}
	if pe.Class != pterr.ClassConfig {
		t.Fatalf("error Class = %v, want ClassConfig", pe.Class)
	}
}
