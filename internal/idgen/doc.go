// Package idgen produces unguessable identifiers for PhantomTraffic.
//
// SECURITY CONTRACT (AGENTS.md §2.2; design §7): this package is the ONLY
// place that mints security-sensitive identifiers — session ids, correlation
// /request ids, the stable per-host AgentID, and generic nonces. It draws
// entropy EXCLUSIVELY from crypto/rand. math/rand and math/rand/v2 are
// FORBIDDEN here and the CI forbidigo lint enforces that boundary: the
// non-cryptographic shaping RNG (math/rand/v2) lives only in internal/rng and
// internal/behavior and must never produce an id, key, nonce, or token.
//
// All identifiers are encoded with base64.RawURLEncoding (URL-safe, unpadded)
// so they are safe in URLs, filenames, structured-log fields, and HTTP headers
// without further escaping.
package idgen
