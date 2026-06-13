// Package cookie holds the per-session-group cookie jar registry referenced by
// the determinism seam (SharedState.JarGroups). It is internally mutex-safe: it
// is part of the one acknowledged shared-mutable region (design §2, §5).
//
// STUB (Plan 1): the Registry type below is a minimal stand-in so the
// internal/protocols SharedState seam compiles. The concrete, internally-locked
// jar registry (jar lookup/creation keyed by session-group, cookie set/get
// honoring the navigation allowlist) lands in the cookie plan and will replace
// this. Keep this surface empty until then.
package cookie

// Registry holds cookie jars keyed by session-group. STUB (Plan 1): an opaque,
// internally-synchronized handle the contract layer carries by pointer on
// SharedState.JarGroups; the cookie plan owns the real implementation and its
// jar lookup/mutation methods.
type Registry struct {
	// Intentionally empty in Plan 1. The cookie plan adds the synchronized jar
	// map and accessor methods. SharedState carries this by pointer
	// (*cookie.Registry), matching the design's mutex-safe-shared contract.
}
