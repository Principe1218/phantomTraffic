// Package dnscache holds the process-wide, internally-synchronized DNS resolver
// cache referenced by the determinism seam (SharedState.DNSCache). TTL aging is
// driven by the injected clock so a fake clock can replay/expire entries
// deterministically (design §2, §5).
//
// STUB (Plan 1): the Cache interface below is a minimal stand-in so the
// internal/protocols SharedState seam compiles. The concrete, mutex-safe
// implementation — and the full Entry shape — land in the dnscache plan and
// must match the design's Lookup(qname,qtype)->(Entry,bool) / Store(...) shape
// (reviewer note #3). Reconcile field/method names when that plan lands.
package dnscache

import "time"

// Entry is a single cached resolution slice. STUB (Plan 1): fields are the
// minimum the contract layer needs to type the interface; the dnscache plan
// owns the authoritative shape.
type Entry struct {
	QName      string    // queried name
	QType      string    // queried record type (A, AAAA, CNAME, …)
	Addrs      []string  // resolved addresses (value copy; never aliased into the cache)
	TTLExpires time.Time // when this entry ages out, against the injected clock
}

// Cache is the internally-synchronized DNS cache surface (design §2). Every
// method MUST be safe for concurrent use: it is part of the one acknowledged
// shared-mutable region (SharedState). STUB (Plan 1): replaced by the dnscache
// plan's concrete implementation.
type Cache interface {
	// Lookup returns the cached entry for (qname, qtype) if present and unexpired.
	Lookup(qname, qtype string) (Entry, bool)
	// Store records a resolution; the cache copies what it needs (no aliasing).
	Store(e Entry)
}
