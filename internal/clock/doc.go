// Package clock is the injected time seam. Every clock read in the system goes
// through clock.Clock so a fake clock can fast-forward a multi-hour schedule in
// microseconds and freeze logical time uniformly during pause.
//
// See docs/superpowers/specs/2026-06-11-core-foundation-design.md Section 2.
package clock
