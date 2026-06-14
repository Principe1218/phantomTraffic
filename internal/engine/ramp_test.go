package engine

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// TestConcurrencyAt covers the pure ramp function: no-ramp passthrough, the elapsed
// edges (<=0 and >=Up), and the linear interior with rounding and clamping.
func TestConcurrencyAt(t *testing.T) {
	cases := []struct {
		name    string
		plan    scenario.RampPlan
		target  int
		elapsed time.Duration
		want    int
	}{
		{
			name:    "up<=0 returns target (instant ramp)",
			plan:    scenario.RampPlan{Up: 0, StartConcurrency: 1},
			target:  10,
			elapsed: 0,
			want:    10,
		},
		{
			name:    "up<=0 returns target even mid-window",
			plan:    scenario.RampPlan{Up: 0, StartConcurrency: 1},
			target:  10,
			elapsed: 5 * time.Second,
			want:    10,
		},
		{
			name:    "elapsed<=0 returns max(1, StartConcurrency)",
			plan:    scenario.RampPlan{Up: time.Minute, StartConcurrency: 3},
			target:  10,
			elapsed: 0,
			want:    3,
		},
		{
			name:    "elapsed negative returns max(1, StartConcurrency)",
			plan:    scenario.RampPlan{Up: time.Minute, StartConcurrency: 3},
			target:  10,
			elapsed: -time.Second,
			want:    3,
		},
		{
			name:    "elapsed<=0 floors StartConcurrency at 1",
			plan:    scenario.RampPlan{Up: time.Minute, StartConcurrency: 0},
			target:  10,
			elapsed: 0,
			want:    1,
		},
		{
			name:    "elapsed>=Up returns target",
			plan:    scenario.RampPlan{Up: time.Minute, StartConcurrency: 1},
			target:  10,
			elapsed: time.Minute,
			want:    10,
		},
		{
			name:    "elapsed past Up returns target",
			plan:    scenario.RampPlan{Up: time.Minute, StartConcurrency: 1},
			target:  10,
			elapsed: 2 * time.Minute,
			want:    10,
		},
		{
			name:    "linear midpoint rounds 1..10 at half => 6",
			plan:    scenario.RampPlan{Up: 10 * time.Second, StartConcurrency: 1},
			target:  10,
			elapsed: 5 * time.Second,
			want:    6, // 1 + (10-1)*0.5 = 5.5 -> round half up -> 6
		},
		{
			name:    "linear quarter 1..10 => 3",
			plan:    scenario.RampPlan{Up: 10 * time.Second, StartConcurrency: 1},
			target:  10,
			elapsed: 2500 * time.Millisecond,
			want:    3, // 1 + 9*0.25 = 3.25 -> 3
		},
		{
			name:    "linear start is exactly StartConcurrency",
			plan:    scenario.RampPlan{Up: 10 * time.Second, StartConcurrency: 2},
			target:  8,
			elapsed: time.Nanosecond, // elapsed > 0 so we are in the interior
			want:    2,               // ~no progress yet, clamps to start
		},
		{
			name:    "interior never exceeds target (clamp high)",
			plan:    scenario.RampPlan{Up: 10 * time.Second, StartConcurrency: 5},
			target:  6,
			elapsed: 9 * time.Second,
			want:    6, // 5 + (6-5)*0.9 = 5.9 -> 6, never above target
		},
		{
			name:    "interior never drops below 1 (clamp low)",
			plan:    scenario.RampPlan{Up: 10 * time.Second, StartConcurrency: 1},
			target:  1,
			elapsed: 1 * time.Second,
			want:    1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := concurrencyAt(tc.plan, tc.target, tc.elapsed)
			if got != tc.want {
				t.Fatalf("concurrencyAt(%+v, %d, %s) = %d, want %d",
					tc.plan, tc.target, tc.elapsed, got, tc.want)
			}
		})
	}
}
