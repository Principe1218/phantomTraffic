package protocols

// secretRef is a TEMPORARY local stand-in for secret.CredentialRef so this
// package compiles in isolation during TDD before internal/secret lands on
// the module path. internal/secret.CredentialRef is an OPAQUE handle that
// carries NO secret bytes (design §2, AGENTS.md §3.1). This shim mirrors that:
// an opaque, comparable identifier with no secret material.
//
// DELETE THIS FILE and import internal/secret once that module is wired (see
// the session.go task). It exists only to keep the contract-layer TDD green
// without coupling task ordering across modules.
type secretRef struct {
	id string // opaque credential id; resolved lazily inside the handler
}
