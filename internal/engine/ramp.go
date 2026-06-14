package engine

import (
	"math"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// concurrencyAt is the pure ramp governor function: the live concurrency target at a
// given elapsed offset from the start of a block. The ramp governor (Module E5) ticks
// a clock.Timer and feeds the elapsed value here, then calls sem.setLimit with the
// result. Behavior:
//
//   - plan.Up <= 0       => no ramp; the block runs at full target immediately.
//   - elapsed <= 0       => the start floor, max(1, plan.StartConcurrency).
//   - elapsed >= plan.Up => full target.
//   - otherwise          => linear interpolation StartConcurrency..target by
//     elapsed/Up, rounded half-up, clamped to [1, target].
func concurrencyAt(plan scenario.RampPlan, target int, elapsed time.Duration) int {
	if plan.Up <= 0 {
		return target
	}

	start := plan.StartConcurrency
	if start < 1 {
		start = 1
	}
	if elapsed <= 0 {
		return start
	}
	if elapsed >= plan.Up {
		return target
	}

	frac := float64(elapsed) / float64(plan.Up)
	n := int(math.Round(float64(start) + float64(target-start)*frac))

	if n < 1 {
		n = 1
	}
	if n > target {
		n = target
	}
	return n
}
