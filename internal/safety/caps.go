package safety

import "time"

// Caps is the effective (merged, most-restrictive) per-session safety cap set
// carried on SessionDeps (design §2, §6). It bounds how aggressively a single
// virtual user may act: caps may be LOWERED freely by config but RAISED only
// via the audited override flag.
//
// STUB (Plan 1): this is a minimal stand-in so the internal/protocols
// determinism seam (SessionDeps.Caps) compiles. The full safety package — the
// two-tier token-bucket Limiter, Reservation reconcile/release, and the audited
// raise-override path described in safety/doc.go — lands in its own later plan
// and will replace/extend these fields. Keep this surface small until then.
type Caps struct {
	MaxConcurrent  int           // ceiling on in-flight actions for this session (0 = unset)
	MaxRPS         float64       // per-session request-rate ceiling (0 = unset)
	PerActionLimit time.Duration // ceiling on a single action's wall time (0 = unset)
}
