// Package rng provides an injectable, NON-CRYPTOGRAPHIC pseudo-random source for
// traffic-timing, ordering, and fingerprint shaping. It wraps math/rand/v2 (PCG).
//
// SECURITY (AGENTS.md §2.2 / design §7): this generator is non-cryptographic by
// design and MUST NEVER produce keys, nonces, tokens, or session ids — those go
// through internal/idgen (crypto/rand only). A CI forbidigo/gosec lint forbids
// math/rand and math/rand/v2 imports anywhere except internal/rng and
// internal/behavior. This package is one of those two permitted boundaries.
package rng

import "math/rand/v2" // nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used

// Rand is the injectable random source. Signatures are copied verbatim from the
// core foundation design, section 2.
//
// SAMPLING CONTRACT (design §2): NormFloat64 and ExpFloat64 DELEGATE to
// math/rand/v2's existing Ziggurat sampler verbatim. We never reimplement a
// numeric sampler (AGENTS.md §2.1). Distributions that consume this interface only
// transform and clamp the delegated draws.
type Rand interface {
	Float64() float64
	NormFloat64() float64 // delegates to math/rand/v2 (Ziggurat)
	ExpFloat64() float64  // delegates to math/rand/v2 (Ziggurat)
	IntN(n int) int
	Int63n(n int64) int64
	Perm(n int) []int
	Split() Rand // per-vuser stream derivation; see prodRand.Split for the mechanism
}

// prodRand is the production Rand. It wraps a *math/rand/v2.Rand backed by a PCG
// source and delegates every method to the standard library.
type prodRand struct {
	r *rand.Rand
}

// New returns a production Rand seeded from the two given 64-bit seed words,
// backed by a math/rand/v2 PCG generator. Equal (seed1, seed2) pairs yield equal
// sequences, which is the foundation of single-agent run replay (design §4).
func New(seed1, seed2 uint64) Rand {
	return &prodRand{r: rand.New(rand.NewPCG(seed1, seed2))}
}

func (p *prodRand) Float64() float64 { return p.r.Float64() }

// NormFloat64 delegates to math/rand/v2's Ziggurat sampler (no reimplementation).
func (p *prodRand) NormFloat64() float64 { return p.r.NormFloat64() }

// ExpFloat64 delegates to math/rand/v2's Ziggurat sampler (no reimplementation).
func (p *prodRand) ExpFloat64() float64 { return p.r.ExpFloat64() }

func (p *prodRand) IntN(n int) int { return p.r.IntN(n) }

// Int63n forwards to math/rand/v2's Int64N (v2 renamed Int63n to Int64N; the
// design keeps the historical name on the interface for call-site familiarity).
func (p *prodRand) Int63n(n int64) int64 { return p.r.Int64N(n) }

func (p *prodRand) Perm(n int) []int { return p.r.Perm(n) }

// Split derives an independent, reproducible child stream. math/rand/v2 exposes no
// native Split on *Rand or PCG, so per the design (§2.1) we draw two full 64-bit
// generator words from the PARENT and use them as the seeds of a FRESH PCG-backed
// child. Drawing whole Uint64 words straight from the parent generator (rather than
// hashing/xoring state by hand) is the documented derivation mechanism and is NOT
// hand-rolled bit-mixing. Because the seeds come deterministically from the parent's
// stream, the same parent seed reproduces the same child stream; because the parent
// then advances past those two words, parent and child diverge.
func (p *prodRand) Split() Rand {
	s1 := p.r.Uint64()
	s2 := p.r.Uint64()
	return &prodRand{r: rand.New(rand.NewPCG(s1, s2))}
}
