package behavior

import (
	"testing"
	"time"
)

func mostlyOnes() [24]float64 {
	var a [24]float64
	for i := range a {
		a[i] = 1
	}
	return a
}

func TestPiecewiseCurveInterpolatesAndSelectsDayType(t *testing.T) {
	weekday := mostlyOnes()
	weekday[10] = 0.5
	weekday[11] = 1.0 // 10:30 interpolates to 0.75
	weekday[3] = 0.0  // 3am clamps to the floor, never zero
	weekend := mostlyOnes()
	weekend[10] = 0.2 // distinct from weekday so day-type selection is observable

	c := PiecewiseCurve{Loc: time.UTC, Weekday: weekday, Weekend: weekend}

	// Monday 10:30 -> interpolate 0.5..1.0 by half an hour -> 0.75
	mon := time.Date(2026, 1, 5, 10, 30, 0, 0, time.UTC)
	if got := c.Intensity(mon); got != 0.75 {
		t.Fatalf("Mon 10:30 intensity = %v, want 0.75", got)
	}
	// Monday 03:00 -> 0.0 table value clamps to the floor
	mon3 := time.Date(2026, 1, 5, 3, 0, 0, 0, time.UTC)
	if got := c.Intensity(mon3); got != intensityFloor {
		t.Fatalf("Mon 03:00 intensity = %v, want floor %v", got, intensityFloor)
	}
	// Saturday 10:00 -> weekend table (0.2), proving day-type selection
	sat := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC) // a Saturday
	if got := c.Intensity(sat); got != 0.2 {
		t.Fatalf("Sat 10:00 intensity = %v, want 0.2", got)
	}
}

func TestFlatTimeOfDayIsPeak(t *testing.T) {
	if got := (FlatTimeOfDay{}).Intensity(base); got != 1 {
		t.Fatalf("FlatTimeOfDay intensity = %v, want 1", got)
	}
}
