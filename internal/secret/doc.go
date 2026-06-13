// Package secret provides the opaque CredentialRef handle (never secret bytes)
// and the Secret wrapper that redacts via slog.LogValuer, fmt.Stringer/GoStringer,
// and json.Marshaler — so a credential can never be formatted into a log line.
// Secrets are resolved lazily inside handlers via a CredentialSource; engine and
// behavior hold only the interface and a ref.
//
// See docs/superpowers/specs/2026-06-11-core-foundation-design.md Section 7.
package secret
