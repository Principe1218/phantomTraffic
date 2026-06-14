package schedule_test

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/schedule"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// mustLoc loads a named location or fails the test; the Go time zone database is
// available on every supported platform / CI runner.
func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

// hm is a small helper: a since-midnight duration for HH:MM.
func hm(h, m int) time.Duration {
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute
}

// weekdays returns a Days mask covering Mon..Fri (time.Weekday: Sun=0..Sat=6).
func weekdays() [7]bool {
	var d [7]bool
	d[time.Monday], d[time.Tuesday], d[time.Wednesday] = true, true, true
	d[time.Thursday], d[time.Friday] = true, true
	return d
}

// weekends returns a Days mask covering Sat+Sun.
func weekends() [7]bool {
	var d [7]bool
	d[time.Saturday], d[time.Sunday] = true, true
	return d
}

func TestActive(t *testing.T) {
	ny := mustLoc(t, "America/New_York")

	// 2026-06-15 is a Monday; 2026-06-13 is a Saturday (en-US calendar).
	officeHours := scenario.Schedule{
		Loc: ny,
		Windows: []scenario.ScheduleWindow{
			{Days: weekdays(), Start: hm(8, 0), End: hm(18, 0)},
		},
	}
	weekendWindow := scenario.Schedule{
		Loc: ny,
		Windows: []scenario.ScheduleWindow{
			{Days: weekends(), Start: hm(9, 0), End: hm(17, 0)},
		},
	}

	tests := []struct {
		name string
		sch  scenario.Schedule
		at   time.Time
		want bool
	}{
		{
			name: "empty windows always active",
			sch:  scenario.Schedule{Loc: ny},
			at:   time.Date(2026, 6, 15, 3, 0, 0, 0, ny), // 3am Monday, no windows
			want: true,
		},
		{
			name: "weekday inside window",
			sch:  officeHours,
			at:   time.Date(2026, 6, 15, 12, 0, 0, 0, ny), // noon Monday
			want: true,
		},
		{
			name: "weekday before window",
			sch:  officeHours,
			at:   time.Date(2026, 6, 15, 7, 59, 0, 0, ny),
			want: false,
		},
		{
			name: "weekday after window",
			sch:  officeHours,
			at:   time.Date(2026, 6, 15, 18, 0, 1, 0, ny),
			want: false,
		},
		{
			name: "weekday on start boundary is active",
			sch:  officeHours,
			at:   time.Date(2026, 6, 15, 8, 0, 0, 0, ny), // [Start, End)
			want: true,
		},
		{
			name: "weekday on end boundary is inactive",
			sch:  officeHours,
			at:   time.Date(2026, 6, 15, 18, 0, 0, 0, ny), // End is exclusive
			want: false,
		},
		{
			name: "saturday excluded by weekday window",
			sch:  officeHours,
			at:   time.Date(2026, 6, 13, 12, 0, 0, 0, ny), // Saturday noon
			want: false,
		},
		{
			name: "weekend window active on saturday",
			sch:  weekendWindow,
			at:   time.Date(2026, 6, 13, 12, 0, 0, 0, ny), // Saturday noon
			want: true,
		},
		{
			name: "weekend window inactive on monday",
			sch:  weekendWindow,
			at:   time.Date(2026, 6, 15, 12, 0, 0, 0, ny), // Monday noon
			want: false,
		},
		{
			name: "evaluates in schedule loc not instant loc",
			sch:  officeHours,
			// 16:00 UTC on Monday == 12:00 EDT (UTC-4) == inside the NY window.
			at:   time.Date(2026, 6, 15, 16, 0, 0, 0, time.UTC),
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := schedule.Active(tc.sch, tc.at); got != tc.want {
				t.Errorf("Active(%s) = %v, want %v", tc.at.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

func TestNextTransition(t *testing.T) {
	ny := mustLoc(t, "America/New_York")

	officeHours := scenario.Schedule{
		Loc: ny,
		Windows: []scenario.ScheduleWindow{
			{Days: weekdays(), Start: hm(8, 0), End: hm(18, 0)},
		},
	}

	// A two-window weekday: morning 08:00-12:00 and afternoon 13:00-17:00.
	splitDay := scenario.Schedule{
		Loc: ny,
		Windows: []scenario.ScheduleWindow{
			{Days: weekdays(), Start: hm(8, 0), End: hm(12, 0)},
			{Days: weekdays(), Start: hm(13, 0), End: hm(17, 0)},
		},
	}

	tests := []struct {
		name string
		sch  scenario.Schedule
		at   time.Time
		want time.Time
	}{
		{
			name: "no windows returns zero time",
			sch:  scenario.Schedule{Loc: ny},
			at:   time.Date(2026, 6, 15, 12, 0, 0, 0, ny),
			want: time.Time{},
		},
		{
			name: "before window flips at start",
			sch:  officeHours,
			at:   time.Date(2026, 6, 15, 6, 0, 0, 0, ny), // 6am Monday
			want: time.Date(2026, 6, 15, 8, 0, 0, 0, ny), // 8am Monday
		},
		{
			name: "inside window flips at end",
			sch:  officeHours,
			at:   time.Date(2026, 6, 15, 12, 0, 0, 0, ny), // noon Monday
			want: time.Date(2026, 6, 15, 18, 0, 0, 0, ny), // 6pm Monday
		},
		{
			name: "exactly at start (active) flips at end",
			sch:  officeHours,
			at:   time.Date(2026, 6, 15, 8, 0, 0, 0, ny),
			want: time.Date(2026, 6, 15, 18, 0, 0, 0, ny),
		},
		{
			name: "after friday window flips at monday start",
			sch:  officeHours,
			at:   time.Date(2026, 6, 19, 19, 0, 0, 0, ny), // 7pm Friday
			want: time.Date(2026, 6, 22, 8, 0, 0, 0, ny),  // 8am Monday
		},
		{
			name: "split day midday gap flips at afternoon start",
			sch:  splitDay,
			at:   time.Date(2026, 6, 15, 12, 30, 0, 0, ny), // in the 12:00-13:00 gap
			want: time.Date(2026, 6, 15, 13, 0, 0, 0, ny),
		},
		{
			name: "split day morning flips at morning end",
			sch:  splitDay,
			at:   time.Date(2026, 6, 15, 9, 0, 0, 0, ny),
			want: time.Date(2026, 6, 15, 12, 0, 0, 0, ny),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := schedule.NextTransition(tc.sch, tc.at)
			if !got.Equal(tc.want) {
				t.Fatalf("NextTransition(%s) = %s, want %s",
					tc.at.Format(time.RFC3339), got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
			if tc.want.IsZero() {
				return // no edge to validate
			}
			if got.Before(tc.at) {
				t.Fatalf("NextTransition returned %s before t=%s", got, tc.at)
			}
			// The active state must genuinely flip across the returned edge.
			before := schedule.Active(tc.sch, got.Add(-time.Nanosecond))
			after := schedule.Active(tc.sch, got)
			if before == after {
				t.Fatalf("Active did not flip across %s: before=%v after=%v", got, before, after)
			}
		})
	}
}
