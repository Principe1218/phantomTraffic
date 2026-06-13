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
