package behavior

import "time"

// TimeOfDayShaper returns a traffic-intensity multiplier in (0,1] for a logical
// time: 1 at the busiest hour, lower when quiet. The shaper stretches think-time
// by dividing by this value, so a low intensity means longer pauses (less
// traffic). Pure function of the injected `now` (design §4).
type TimeOfDayShaper interface {
	Intensity(now time.Time) float64
	Name() string
}

// intensityFloor keeps Intensity strictly positive so the shaper's divide can
// never blow up to infinity on a 0.0 table entry (e.g. 3am).
const intensityFloor = 0.01

// PiecewiseCurve is a 24-point hourly intensity table in a fixed timezone, with
// separate weekday and weekend curves and linear interpolation between hour
// points. "Tuesday 2pm vs Sunday 3am" is a deterministic unit test.
type PiecewiseCurve struct {
	Loc     *time.Location
	Weekday [24]float64
	Weekend [24]float64
}

func (c PiecewiseCurve) Intensity(now time.Time) float64 {
	loc := c.Loc
	if loc == nil {
		loc = time.UTC
	}
	t := now.In(loc)
	table := c.Weekday
	if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
		table = c.Weekend
	}
	hour := t.Hour()
	frac := (float64(t.Minute())*60 + float64(t.Second())) / 3600.0
	a := table[hour]
	b := table[(hour+1)%24]
	return clampIntensity(a + (b-a)*frac)
}

func (c PiecewiseCurve) Name() string { return "piecewise" }

// clampIntensity bounds a curve value to [intensityFloor, 1].
func clampIntensity(v float64) float64 {
	if v < intensityFloor {
		return intensityFloor
	}
	if v > 1 {
		return 1
	}
	return v
}

// FlatTimeOfDay is constant peak intensity (no time-of-day shaping).
type FlatTimeOfDay struct{}

func (FlatTimeOfDay) Intensity(time.Time) float64 { return 1 }
func (FlatTimeOfDay) Name() string                { return "flat" }
