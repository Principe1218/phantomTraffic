package schedule

import (
	"slices"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// Active reports whether t falls inside any configured window. An empty Windows
// slice means "always active" (the engine spawns no scheduler goroutine in that
// case). Windows evaluate in s.Loc; the local weekday selects the Days mask and
// the since-midnight offset is compared against [Start, End) — End is exclusive so
// adjacent windows never double-count a boundary instant.
func Active(s scenario.Schedule, t time.Time) bool {
	if len(s.Windows) == 0 {
		return true
	}
	local := t.In(s.Loc)
	day := local.Weekday()
	off := sinceMidnight(local)
	for _, w := range s.Windows {
		if w.Days[day] && off >= w.Start && off < w.End {
			return true
		}
	}
	return false
}

// sinceMidnight returns the duration from local midnight to t, in t's own
// location. It is computed from clock fields (not t.Sub of a midnight Time) so it
// is unaffected by DST offset shifts: the window math is defined in local wall-clock
// terms, matching how operators read "08:00..18:00".
func sinceMidnight(t time.Time) time.Duration {
	h, m, sec := t.Clock()
	return time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(t.Nanosecond())
}

// maxLookaheadDays bounds the forward scan for the next edge. The MVP forbids
// cross-midnight windows but allows any combination of weekdays, so a transition is
// always found within one week if any window exists; 8 days gives a one-day margin
// and guards against an all-false Days mask (which yields the zero time).
const maxLookaheadDays = 8

// NextTransition returns the next instant at or after t at which the active state
// flips. With no windows the schedule is always active and never flips, so it
// returns the zero time.Time. Otherwise it scans the sorted edges (each window
// contributes a Start and an End on each of its days) and returns the earliest edge
// strictly after t that changes Active. The scan is bounded to one week of
// lookahead; if no edge is found (e.g. an all-false Days mask) it returns the zero
// time.
func NextTransition(s scenario.Schedule, t time.Time) time.Time {
	if len(s.Windows) == 0 {
		return time.Time{}
	}
	local := t.In(s.Loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.Loc)

	cur := Active(s, t)
	for day := 0; day < maxLookaheadDays; day++ {
		base := midnight.AddDate(0, 0, day)
		for _, edge := range dayEdges(s, base) {
			if !edge.After(t) {
				continue
			}
			if Active(s, edge) != cur {
				return edge
			}
		}
	}
	return time.Time{}
}

// dayEdges returns the sorted window boundary instants for the calendar day that
// begins at base (local midnight in s.Loc). Each window active on base's weekday
// contributes its Start and End as absolute instants. Boundaries are added in
// window order then sorted ascending so NextTransition's scan visits them in time
// order. End is exclusive in Active, so an End edge is where active->inactive.
func dayEdges(s scenario.Schedule, base time.Time) []time.Time {
	day := base.Weekday()
	edges := make([]time.Time, 0, len(s.Windows)*2)
	for _, w := range s.Windows {
		if !w.Days[day] {
			continue
		}
		edges = append(edges, base.Add(w.Start), base.Add(w.End))
	}
	slices.SortFunc(edges, func(a, b time.Time) int { return a.Compare(b) })
	return edges
}
