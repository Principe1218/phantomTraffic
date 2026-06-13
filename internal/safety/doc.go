// Package safety holds the non-bypassable safety primitives referenced by the
// determinism seam: the Caps value type carried on every Session, the two-tier
// (per-target AND global) token-bucket Limiter over the injected clock, and
// Reservation reconcile/release. Caps may be lowered freely by config but raised
// only via the audited override flag.
//
// See docs/superpowers/specs/2026-06-11-core-foundation-design.md Sections 2, 6.
package safety
