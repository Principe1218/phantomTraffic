// Package engine turns a frozen scenario.Scenario plus Plan-3 behavior.Sessions
// into real, paced, safety-capped, cancelable execution: the supervisor tree,
// vuser population, dispatcher, ramp governor, dual-source pause/resume gate,
// sharded stats, the error-taxonomy reactions, and Run.ApplyPatch live
// reconfiguration described in the core-foundation design §5/§6/§8.
//
// DETERMINISM BOUNDARY: this package is the engine, but it is NOT the clock. All
// sleeping, timers, and "now" come from the injected clock.Clock; all randomness
// (rotation, backoff jitter) comes from the injected rng.Rand; correlation/run
// identity comes from internal/idgen (crypto/rand). The engine never calls
// time.Now, time.Sleep, or any global rand function, and never imports math/rand.
// CI lint (forbidigo) enforces this boundary on internal/engine, and depguard
// bans math/rand here. A fake clock plus a seeded/scripted rng make a long run
// reproducible byte-for-byte in microseconds.
//
// CONCURRENCY: one goroutine per vuser session; the only shared-mutable hot-path
// state is the atomic per-target stats shards. The rotation selector, the
// limiter, and the semaphore are internally synchronized. No handler runs on a
// bare `go`; every goroutine joins the supervisor's WaitGroup.
//
// See docs/superpowers/specs/2026-06-13-plan-4-engine-runtime-design.md and the
// authoritative parent 2026-06-11-core-foundation-design.md §5/§6/§7/§8.
package engine
