package pterr

import "strings"

// FieldError is one config-validation problem tied to a specific YAML field
// path. Field is the user-facing key (e.g. "name", "scenarios[1].id") so an
// operator can find and fix the exact line; Msg is redaction-safe (no secrets,
// no raw user payloads beyond the offending value). Always ClassConfig.
type FieldError struct {
	Field string
	Msg   string
}

// Error renders the field path and message as "<field>: <msg>".
func (e FieldError) Error() string { return e.Field + ": " + e.Msg }

// Class reports the pterr classification for a config/validation failure.
func (e FieldError) Class() Class { return ClassConfig }

// FieldErrors aggregates every FieldError from a validation pass, in
// deterministic discovery order.
type FieldErrors []FieldError

// Error joins every contained FieldError with "; " into one aggregated message.
func (es FieldErrors) Error() string {
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "; ")
}
