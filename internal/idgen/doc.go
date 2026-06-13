// Package idgen generates unguessable identifiers using crypto/rand ONLY:
// session ids, correlation/request ids, the per-host AgentID, and nonces. It
// must never import math/rand or math/rand/v2 (CI depguard enforces this).
// Non-security shaping randomness lives in internal/rng instead.
//
// See docs/superpowers/specs/2026-06-11-core-foundation-design.md Section 7.
package idgen
