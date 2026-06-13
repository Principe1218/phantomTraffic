// Package pterr is the dependency-free error taxonomy. Handlers map their long
// tail of concrete failures onto a closed class set (Transient, Permanent,
// Config, Safety, Unknown) at the handler boundary, so engine and behavior
// branch on a few cases. *pterr.Error carries a born-redacted Msg safe for
// logs/UI; the underlying cause stays server-side only.
//
// See docs/superpowers/specs/2026-06-11-core-foundation-design.md Section 6.
package pterr
