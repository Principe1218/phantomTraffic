// Package scenario loads and validates a PhantomTraffic scenario file into an
// immutable, FROZEN Scenario. Validation is a PURE function (no network, no
// filesystem beyond the caller-supplied path at Load time, no runtime limiter,
// no audit writes). Every validation failure classifies as pterr.ClassConfig
// and is aggregated so a single Validate pass reports ALL problems at once
// (design §5.2, AGENTS.md §5.5: fail fast, fully, at config time).
package scenario

import (
	"strings"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// FieldError is one validation problem tied to a specific YAML field path. The
// Field is the user-facing key (e.g. "name", "scenarios[1].id",
// "caps.per_target_rps") so an operator can find and fix the exact line; the Msg
// is redaction-safe (no secrets, no raw user payloads beyond the offending
// value). Every FieldError classifies as pterr.ClassConfig.
type FieldError struct {
	Field string
	Msg   string
}

// Error renders the field path and message as "<field>: <msg>".
func (e FieldError) Error() string {
	return e.Field + ": " + e.Msg
}

// Class reports the pterr classification for a config/validation failure. It is
// always pterr.ClassConfig so the caller can branch on the closed taxonomy
// without inspecting the concrete type.
func (e FieldError) Class() pterr.Class {
	return pterr.ClassConfig
}

// ValidationErrors is the aggregated result of a Validate pass: every field
// problem found, in deterministic discovery order. An empty ValidationErrors
// means "valid"; Validate converts that to a nil error so a caller's
// `if err != nil` check is correct.
type ValidationErrors []FieldError

// Error joins every contained FieldError with "; " into one aggregated message.
func (v ValidationErrors) Error() string {
	parts := make([]string, len(v))
	for i, fe := range v {
		parts[i] = fe.Error()
	}
	return strings.Join(parts, "; ")
}
