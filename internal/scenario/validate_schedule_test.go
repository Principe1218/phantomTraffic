package scenario

import (
    "testing"
    "time"
)

func TestValidateNilScheduleIsAlwaysActive(t *testing.T) {
    raw := baseRaw()
    raw.Scenarios[0].Concurrency = 1
    raw.Scenarios[0].DurationMinutes = 5
    raw.Schedule = nil
    sc, err := Validate(raw, Options{AgentCount: 1})
    if err != nil {
        t.Fatalf("Validate returned error: %v", err)
    }
    assertEqual(t, "nil-schedule Loc is nil", sc.Schedule.Loc == nil, true)
    assertEqual(t, "nil-schedule Windows len", len(sc.Schedule.Windows), 0)
}

func TestValidateValidScheduleBuilds(t *testing.T) {
    raw := baseRaw()
    raw.Scenarios[0].Concurrency = 1
    raw.Scenarios[0].DurationMinutes = 5
    raw.Schedule = &RawSchedule{
        Timezone: "America/New_York",
        Windows: []RawWindow{
            {Days: []string{"mon", "fri"}, Start: "08:00", End: "18:30"},
        },
    }
    sc, err := Validate(raw, Options{AgentCount: 1})
    if err != nil {
        t.Fatalf("Validate returned error: %v", err)
    }
    assertEqual(t, "Schedule.Loc.String()", sc.Schedule.Loc.String(), "America/New_York")
    assertEqual(t, "len(Schedule.Windows)", len(sc.Schedule.Windows), 1)
    w := sc.Schedule.Windows[0]
    assertEqual(t, "window Days[Monday]", w.Days[int(time.Monday)], true)
    assertEqual(t, "window Days[Friday]", w.Days[int(time.Friday)], true)
    assertEqual(t, "window Days[Tuesday]", w.Days[int(time.Tuesday)], false)
    assertEqual(t, "window Start", w.Start, 8*time.Hour)
    assertEqual(t, "window End", w.End, 18*time.Hour+30*time.Minute)
}

func TestValidateScheduleErrors(t *testing.T) {
    tests := []struct {
        name      string
        sched     RawSchedule
        wantField string
        wantSub   string
    }{
        {
            name:      "unknown timezone",
            sched:     RawSchedule{Timezone: "Mars/Olympus", Windows: []RawWindow{{Days: []string{"mon"}, Start: "08:00", End: "09:00"}}},
            wantField: "schedule.timezone",
            wantSub:   "load",
        },
        {
            name:      "empty days",
            sched:     RawSchedule{Timezone: "UTC", Windows: []RawWindow{{Days: nil, Start: "08:00", End: "09:00"}}},
            wantField: "schedule.windows[0].days",
            wantSub:   "at least one",
        },
        {
            name:      "unknown weekday",
            sched:     RawSchedule{Timezone: "UTC", Windows: []RawWindow{{Days: []string{"funday"}, Start: "08:00", End: "09:00"}}},
            wantField: "schedule.windows[0].days[0]",
            wantSub:   "unknown day",
        },
        {
            name:      "malformed start",
            sched:     RawSchedule{Timezone: "UTC", Windows: []RawWindow{{Days: []string{"mon"}, Start: "8am", End: "09:00"}}},
            wantField: "schedule.windows[0].start",
            wantSub:   "HH:MM",
        },
        {
            name:      "malformed end",
            sched:     RawSchedule{Timezone: "UTC", Windows: []RawWindow{{Days: []string{"mon"}, Start: "08:00", End: "25:00"}}},
            wantField: "schedule.windows[0].end",
            wantSub:   "HH:MM",
        },
        {
            name:      "end equals start",
            sched:     RawSchedule{Timezone: "UTC", Windows: []RawWindow{{Days: []string{"mon"}, Start: "08:00", End: "08:00"}}},
            wantField: "schedule.windows[0].end",
            wantSub:   "after start",
        },
        {
            name:      "cross-midnight rejected",
            sched:     RawSchedule{Timezone: "UTC", Windows: []RawWindow{{Days: []string{"mon"}, Start: "22:00", End: "02:00"}}},
            wantField: "schedule.windows[0].end",
            wantSub:   "after start",
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            raw := baseRaw()
            raw.Scenarios[0].Concurrency = 1
            raw.Scenarios[0].DurationMinutes = 5
            sched := tt.sched
            raw.Schedule = &sched
            _, err := Validate(raw, Options{AgentCount: 1})
            assertFieldError(t, requireValidationErrors(t, err), tt.wantField, tt.wantSub)
        })
    }
}
