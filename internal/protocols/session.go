package protocols

import (
	"log/slog"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/cookie"
	"github.com/Principe1218/phantomTraffic/internal/dnscache"
	"github.com/Principe1218/phantomTraffic/internal/rng"
	"github.com/Principe1218/phantomTraffic/internal/safety"
	"github.com/Principe1218/phantomTraffic/internal/secret"
)

// Session is the virtual user (vuser): persona-scoped and MULTI-PROTOCOL
// (design §1 Amendment 2, §2). One vuser's Session interleaves actions across
// different handlers over its lifetime; States holds one lazily-opened
// SessionState per protocol the vuser has touched. The injected dependencies
// (the determinism seam) are carried on Deps and never reached via globals.
type Session struct {
	ID      SessionID
	Persona string                      // label only; never a secret
	Targets TargetSet                   // resolved + FROZEN at validate; the navigation allowlist
	States  map[ProtocolID]SessionState // lazily opened; one per protocol this vuser has touched
	Deps    SessionDeps
}

// SessionDeps is THE determinism seam (design §2). Carried on every Session.
// Clock and Rand are the only sources of time and randomness; a fake clock
// fast-forwards a 10-hour schedule and a seeded/split Rand replays a run.
// Rand is the NON-crypto shaping RNG (math/rand/v2 via internal/rng) — crypto
// draws (session ids, nonces) go through internal/idgen, NEVER here
// (AGENTS.md §2.2). Log is pre-tagged and never passed secrets; CredSrc
// resolves a CredentialRef to key bytes LAZILY, inside the handler only.
type SessionDeps struct {
	Clock     clock.Clock
	Rand      rng.Rand     // non-crypto shaping RNG (§7)
	Log       *slog.Logger // pre-tagged with session/request id; never passed secrets
	Stats     StatsRecorder
	Caps      safety.Caps               // effective (merged, most-restrictive) caps for this session
	Audit     audit.Sink                // append-only local audit log for security-relevant events (§7)
	CredSrc   secret.CredentialResolver // resolves CredentialRef -> key bytes LAZILY, inside handler only
	Shared    SharedState               // process-wide caches the realism needs
	Transport TransportProbe            // injectable byte/throughput source — REAL in prod, scripted in tests (§8)
}

// SharedState is the ONE acknowledged concurrent-shared-mutable region in the
// system (design §2). EVERY type in it MUST be internally synchronized; it is
// the documented exception to the single-goroutine-per-session invariant.
type SharedState struct {
	DNSCache  dnscache.Cache   // interface; internally synchronized; injected Clock for TTL aging
	JarGroups *cookie.Registry // cookie jars keyed by session-group; internally mutex-safe
}

// StatsRecorder receives SCRUBBED Results only (never Observation). It is the
// engine's race-free aggregation entry point; the contract layer depends only
// on the interface so handlers and behavior never import the engine.
type StatsRecorder interface {
	Record(r Result)
}

// TransportProbe is the injectable byte/throughput source (design §2, §8).
// In production it reports real socket bytes against the real Clock; in tests
// a fake probe reports SCRIPTED byte counts against fake-clock advances, so
// "actual throughput" = bytesObserved / Clock.Since(start) is deterministic.
type TransportProbe interface { // NOSONAR — BytesObserved has no natural -er agent noun; TransportProbe is the established domain term
	BytesObserved(sess SessionID) int64
}
