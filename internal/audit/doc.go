// Package audit is the built-in append-only local audit sink for
// security-relevant events (tls.verification_skipped, ssh.host_key_unverified,
// safety.cap_override_enabled, scenario.started/stopped/patched). Records carry
// actor, action, resource, and timestamp, with optional hash-chaining of records
// for integrity (each record embeds the prior record's digest).
//
// See docs/superpowers/specs/2026-06-11-core-foundation-design.md Section 7.
package audit
