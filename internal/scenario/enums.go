// Package scenario defines PhantomTraffic's static scenario configuration: the
// on-disk YAML schema (the Raw* types), the strict size-bounded loader (Load),
// and — in its second half (Module 5) — the pure Validate function that turns a
// decoded Raw into a frozen, validated Scenario.
//
// This package is intentionally static: it performs NO network I/O, drives NO
// traffic, holds NO runtime limiter, writes NO audit records, and reads NO
// credential files. Load touches exactly one local file (rejecting oversized
// ones); validation (Module 5) is a pure function of its inputs.
package scenario

// RotationStrategy selects how a scenario block walks its target list.
// The zero value (RotationSequential) is the default for an empty
// "target_rotation" YAML field, so the iota order is load-bearing.
type RotationStrategy uint8

const (
	// RotationSequential walks targets in declared order (the default).
	RotationSequential RotationStrategy = iota
	// RotationRandom picks the next target uniformly at random.
	RotationRandom
)

// String returns a stable, low-cardinality label. Any out-of-range value maps
// to "unknown" so a corrupt value can never widen the logged surface.
func (r RotationStrategy) String() string {
	switch r {
	case RotationSequential:
		return "sequential"
	case RotationRandom:
		return "random"
	default:
		return "unknown"
	}
}

// ExecutionMode selects whether scenario blocks run together or one after
// another. The zero value (ExecParallel) is the default for an empty
// "execution.mode" YAML field, so the iota order is load-bearing.
type ExecutionMode uint8

const (
	// ExecParallel runs all blocks concurrently (the default).
	ExecParallel ExecutionMode = iota
	// ExecSequential runs blocks one at a time in declared order.
	ExecSequential
)

// String returns a stable, low-cardinality label. Any out-of-range value maps
// to "unknown" so a corrupt value can never widen the logged surface.
func (e ExecutionMode) String() string {
	switch e {
	case ExecParallel:
		return "parallel"
	case ExecSequential:
		return "sequential"
	default:
		return "unknown"
	}
}

// WeightBasis selects how per-block weights translate into load (design §6.7).
// The zero value (WeightByVuserPopulation) is the default for an empty
// "weight_basis" YAML field, so the iota order is load-bearing.
type WeightBasis uint8

const (
	// WeightByVuserPopulation sizes each block's vuser population as a fraction
	// of total concurrency (the foundation §5 default).
	WeightByVuserPopulation WeightBasis = iota
	// WeightByConcurrency weights blocks by their concurrency budget.
	WeightByConcurrency
	// WeightByRequestRate weights blocks by their request rate.
	WeightByRequestRate
)

// String returns a stable, low-cardinality label. Any out-of-range value maps
// to "unknown" so a corrupt value can never widen the logged surface.
func (w WeightBasis) String() string {
	switch w {
	case WeightByVuserPopulation:
		return "vuser_population"
	case WeightByConcurrency:
		return "concurrency"
	case WeightByRequestRate:
		return "request_rate"
	default:
		return "unknown"
	}
}

// parseWeightBasis maps a YAML weight_basis string to a WeightBasis. "" defaults
// to WeightByVuserPopulation (design §6.7). ok is false for an unknown value.
func parseWeightBasis(s string) (WeightBasis, bool) {
	switch s {
	case "", "vuser_population":
		return WeightByVuserPopulation, true
	case "concurrency":
		return WeightByConcurrency, true
	case "request_rate":
		return WeightByRequestRate, true
	default:
		return WeightByVuserPopulation, false
	}
}
