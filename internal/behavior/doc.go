// Package behavior composes protocol actions into human-like multi-protocol
// sessions and owns the Distribution primitives (Constant, Uniform, Normal,
// LogNormal, Exponential) that shape think-time and inter-arrival timing. All
// sampling primitives come from rng.Rand; Distributions only transform and clamp
// the delegated draws — no hand-rolled numeric code.
//
// DETERMINISM: this package is forbidden by CI lint from calling time.Now,
// time.Sleep, or any global rand function. The engine performs all sleeping via
// clock.Clock; randomness flows through the injected rng.Rand. This is one of
// only two packages allowed to import math/rand/v2.
//
// See docs/superpowers/specs/2026-06-11-core-foundation-design.md Sections 2, 4, 7.
package behavior
