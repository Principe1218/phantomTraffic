// Package rng is the NON-CRYPTOGRAPHIC shaping RNG: traffic timing, ordering,
// jitter, burstiness, and UA selection ONLY. It wraps math/rand/v2 (PCG) and
// DELEGATES NormFloat64/ExpFloat64 to the standard library's Ziggurat sampler;
// no numeric sampler is reimplemented. Per-stream derivation uses
// math/rand/v2's built-in Split, never hand-rolled mixing.
//
// SECURITY: never use this for keys, nonces, tokens, or session ids — those go
// through internal/idgen (crypto/rand). This is one of only two packages allowed
// to import math/rand/v2; the CI depguard lint forbids it everywhere else.
//
// See docs/superpowers/specs/2026-06-11-core-foundation-design.md Sections 2, 7.
package rng
