package scenario

import (
    "testing"
    "time"
)

func TestRampPlanZeroAndFields(t *testing.T) {
    // The zero RampPlan means "no ramp" (Up == 0); fields are addressable as pinned.
    var zero RampPlan
    assertEqual(t, "zero RampPlan.Up", zero.Up, time.Duration(0))
    assertEqual(t, "zero RampPlan.StartConcurrency", zero.StartConcurrency, 0)

    p := RampPlan{Up: 60 * time.Second, StartConcurrency: 1}
    assertEqual(t, "RampPlan.Up", p.Up, 60*time.Second)
    assertEqual(t, "RampPlan.StartConcurrency", p.StartConcurrency, 1)
}

func TestScheduleWindowFields(t *testing.T) {
    w := ScheduleWindow{
        Start: 8 * time.Hour,
        End:   18 * time.Hour,
    }
    w.Days[int(time.Monday)] = true
    assertEqual(t, "ScheduleWindow.Start", w.Start, 8*time.Hour)
    assertEqual(t, "ScheduleWindow.End", w.End, 18*time.Hour)
    assertEqual(t, "ScheduleWindow.Days[Monday]", w.Days[int(time.Monday)], true)
    assertEqual(t, "ScheduleWindow.Days[Sunday]", w.Days[int(time.Sunday)], false)
    assertEqual(t, "len(ScheduleWindow.Days)", len(w.Days), 7)
}

func TestScheduleFields(t *testing.T) {
    // Empty Windows is the "always active" representation (design §5).
    var s Schedule
    assertEqual(t, "zero Schedule.Loc is nil", s.Loc == nil, true)
    assertEqual(t, "zero Schedule.Windows len", len(s.Windows), 0)

    s = Schedule{
        Loc:     time.UTC,
        Windows: []ScheduleWindow{{Start: time.Hour, End: 2 * time.Hour}},
    }
    assertEqual(t, "Schedule.Loc", s.Loc, time.UTC)
    assertEqual(t, "len(Schedule.Windows)", len(s.Windows), 1)
}

func TestBlockGainsEngineFields(t *testing.T) {
    b := Block{
        ID:          "web",
        Concurrency: 5,
        Duration:    30 * time.Minute,
        Weight:      70,
        Ramp:        RampPlan{Up: time.Minute, StartConcurrency: 1},
    }
    assertEqual(t, "Block.Concurrency", b.Concurrency, 5)
    assertEqual(t, "Block.Duration", b.Duration, 30*time.Minute)
    assertEqual(t, "Block.Weight", b.Weight, uint(70))
    assertEqual(t, "Block.Ramp.Up", b.Ramp.Up, time.Minute)
    assertEqual(t, "Block.Ramp.StartConcurrency", b.Ramp.StartConcurrency, 1)
}

func TestScenarioGainsEngineFields(t *testing.T) {
    sc := Scenario{
        WeightBasis: WeightByConcurrency,
        Schedule:    Schedule{Loc: time.UTC},
    }
    assertEqual(t, "Scenario.WeightBasis", sc.WeightBasis, WeightByConcurrency)
    assertEqual(t, "Scenario.Schedule.Loc", sc.Schedule.Loc, time.UTC)
}
