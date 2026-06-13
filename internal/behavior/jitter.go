package behavior

import (
	"time"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// JitterModel perturbs a base think-time by a small bounded amount so successive
// pauses are not identical (a constant gap fingerprints a bot). It only transforms
// a delegated rng.Rand draw — no hand-rolled numerics (AGENTS.md §2.1).
type JitterModel interface {
	Jitter(d time.Duration, r rng.Rand) time.Duration
	Name() string
}

// NoJitter returns the duration unchanged. The LogNormal think-time already
// provides human right-skew, so jitter is opt-in on top.
type NoJitter struct{}

func (NoJitter) Jitter(d time.Duration, _ rng.Rand) time.Duration { return d }
func (NoJitter) Name() string                                     { return "none" }

// ProportionalJitter scales d by a factor uniformly drawn from
// [1-Fraction, 1+Fraction] using one Float64 draw. Fraction is clamped to [0,1];
// the result is clamped non-negative. Fraction 0.1 means ±10%.
type ProportionalJitter struct {
	Fraction float64
}

func (j ProportionalJitter) Jitter(d time.Duration, r rng.Rand) time.Duration {
	f := j.Fraction
	if f <= 0 {
		return d
	}
	if f > 1 {
		f = 1
	}
	factor := 1 - f + 2*f*r.Float64() // Float64 ∈ [0,1) -> factor ∈ [1-f, 1+f)
	return clampNonNeg(time.Duration(float64(d) * factor))
}

func (j ProportionalJitter) Name() string { return "proportional" }
