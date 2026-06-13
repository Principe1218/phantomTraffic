// Package protocols holds the core protocol-agnostic contracts: the Action
// vocabulary (marker interface + typed structs + the As[T] downcast), the
// ProtocolHandler interface, the Session/SessionDeps determinism seam, the
// authoritative TargetSet navigation allowlist, the Result/Observation
// envelope, and the handler Registry. Engine and behavior program against
// these interfaces and never import a concrete protocol subpackage.
//
// See docs/superpowers/specs/2026-06-11-core-foundation-design.md Section 2.
package protocols
