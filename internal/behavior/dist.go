// Package behavior holds human-scale realism primitives. Distributions transform
// and clamp draws delegated to the injected rng.Rand; they contain NO hand-rolled
// numeric sampling (AGENTS.md §2.1 — never invent numeric primitives). All random-
// ness flows through rng.Rand, whose NormFloat64/ExpFloat64 delegate to the stdlib
// Ziggurat sampler in internal/rng.
package behavior

import (
	"math"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// Distribution samples a non-negative think-time/inter-arrival duration. Anchored to
// design §2 (core foundation): Sample clamps negative results to 0; Name is a stable
// low-cardinality label for stats/logs.
type Distribution interface {
	Sample(r rng.Rand) time.Duration // non-negative; clamps at 0
	Name() string
}

// clampNonNeg enforces the "non-negative; clamps at 0" contract shared by every impl.
func clampNonNeg(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}

// Constant always samples the same duration. Draws nothing from the RNG.
type Constant struct {
	D time.Duration
}

func (c Constant) Sample(_ rng.Rand) time.Duration { return clampNonNeg(c.D) }
func (c Constant) Name() string                    { return "constant" }

// Uniform samples uniformly in [Min, Max] using one Float64 draw. If Max <= Min it
// degenerates to Min (still clamped non-negative).
type Uniform struct {
	Min, Max time.Duration
}

func (u Uniform) Sample(r rng.Rand) time.Duration {
	span := u.Max - u.Min
	if span <= 0 {
		return clampNonNeg(u.Min)
	}
	// Float64 ∈ [0,1); scale the span and offset by Min. No hand-rolled sampling.
	d := u.Min + time.Duration(r.Float64()*float64(span))
	return clampNonNeg(d)
}

func (u Uniform) Name() string { return "uniform" }

// Normal samples Mean + z*StdDev where z is one rng.Rand.NormFloat64 draw (delegated
// to the stdlib Ziggurat sampler). The result is clamped to [Min, Max] to bound the
// Gaussian tails, then clamped non-negative. No numeric sampling is reimplemented here.
type Normal struct {
	Mean, StdDev, Min, Max time.Duration
}

func (n Normal) Sample(r rng.Rand) time.Duration {
	z := r.NormFloat64()
	d := n.Mean + time.Duration(z*float64(n.StdDev))
	if d < n.Min {
		d = n.Min
	}
	if d > n.Max {
		d = n.Max
	}
	return clampNonNeg(d)
}

func (n Normal) Name() string { return "normal" }

// LogNormal is the default human think-time distribution: right-skewed pauses modeled
// as Scale * exp(Mu + Sigma*z), where z is one rng.Rand.NormFloat64 draw. Mu/Sigma are
// dimensionless log-space parameters; Scale is the duration unit (defaults to one
// second when zero). exp() only transforms the delegated Gaussian draw — no numeric
// sampler is implemented here (AGENTS.md §2.1). exp is always >= 0, but the shared
// non-negative clamp is applied for contract uniformity.
type LogNormal struct {
	Mu, Sigma float64
	Scale     time.Duration
}

func (l LogNormal) Sample(r rng.Rand) time.Duration {
	scale := l.Scale
	if scale == 0 {
		scale = time.Second
	}
	z := r.NormFloat64()
	d := time.Duration(math.Exp(l.Mu+l.Sigma*z) * float64(scale))
	return clampNonNeg(d)
}

func (l LogNormal) Name() string { return "lognormal" }

// Exponential models Poisson inter-arrival gaps as Mean * x, where x is one
// rng.Rand.ExpFloat64 draw (a unit-rate exponential delegated to the stdlib Ziggurat
// sampler). Mean is the desired average gap. The shared non-negative clamp handles a
// negative Mean. No numeric sampler is reimplemented here (AGENTS.md §2.1).
type Exponential struct {
	Mean time.Duration
}

func (e Exponential) Sample(r rng.Rand) time.Duration {
	x := r.ExpFloat64()
	d := time.Duration(x * float64(e.Mean))
	return clampNonNeg(d)
}

func (e Exponential) Name() string { return "exponential" }
