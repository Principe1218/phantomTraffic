package engine

import (
	"github.com/Principe1218/phantomTraffic/internal/safety"
)

// MixWeights maps a block ID to its new relative weight. Re-normalized on apply;
// takes effect at the next vuser recycle (foundation §6.7).
type MixWeights map[string]uint

// TargetSpec is a single target to append to a block's frozen allowlist via the
// same scenario.Validate path. Addr is validated against AllowedDomains on apply.
type TargetSpec struct {
	BlockID string
	Addr    string
}

// ScenarioPatch is a bounded, re-validated, audited live modification of a Run.
// All fields are optional; a nil pointer / empty slice means "leave unchanged".
type ScenarioPatch struct {
	Caps           *safety.CapPatch // DOWN free; UP only under the run's cap-override flag
	Concurrency    *int             // bounded; applied through the resizable semaphore
	Weights        *MixWeights      // re-normalized; effective at next vuser recycle
	RotationIntSec *int             // per-block rotation interval, in seconds
	TargetsAdd     []TargetSpec     // extend the frozen allowlist via the SAME Validate path
	TargetsDisable []string         // soft-disable (breaker force-open); never silent removal
}
