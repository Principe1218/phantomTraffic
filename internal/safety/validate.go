package safety

import "fmt"

// Violation is a single config-time cap problem. Field is the YAML cap key (e.g.
// "per_target_rps") so the caller can point the operator at the exact line; Msg is a
// human-readable, redaction-safe explanation. ValidateCaps returns a slice of these
// rather than an error so the caller (internal/scenario) can fold every cap problem
// into its aggregated FieldError list (AGENTS.md §5.2 — report all input problems).
type Violation struct {
	Field string
	Msg   string
}

// ValidateCaps checks each NON-ZERO declared cap against the ceiling using an
// allowlist range. For each field that is explicitly set (non-zero):
//   - it must be strictly positive (> 0); a non-positive value is ALWAYS a violation,
//     even when override is true — the audited override may raise a cap above the
//     ceiling but may never make it zero-or-negative.
//   - unless override is true, it must also be <= the corresponding ceiling value.
//
// Zero (unset) fields are skipped: they inherit the ceiling via CapSpec.Effective and
// are valid by construction. ValidateCaps returns ALL violations (an empty, non-nil
// slice means valid) and never returns an error. It is pure and deterministic.
func ValidateCaps(declared CapSpec, ceiling Ceiling, override bool) []Violation {
	violations := make([]Violation, 0)

	violations = appendFloatViolation(violations, "per_target_rps", declared.PerTargetRPS, ceiling.PerTargetRPS, override)
	violations = appendFloatViolation(violations, "global_rps", declared.GlobalRPS, ceiling.GlobalRPS, override)
	violations = appendIntViolation(violations, "max_concurrent_sessions", declared.MaxConcurrentSessions, ceiling.MaxConcurrentSessions, override)
	violations = appendInt64Violation(violations, "total_request_budget", declared.TotalRequestBudget, ceiling.TotalRequestBudget, override)
	violations = appendIntViolation(violations, "streaming_byte_rate_kbps", declared.StreamingByteRateKbps, ceiling.StreamingByteRateKbps, override)
	violations = appendIntViolation(violations, "concurrent_streams", declared.ConcurrentStreams, ceiling.ConcurrentStreams, override)
	violations = appendIntViolation(violations, "per_session_max_duration_seconds", declared.PerSessionMaxDurationSeconds, int(ceiling.PerSessionMaxDuration.Seconds()), override)
	violations = appendIntViolation(violations, "per_session_max_actions", declared.PerSessionMaxActions, ceiling.PerSessionMaxActions, override)

	return violations
}

// appendFloatViolation checks one float cap field and appends a Violation if it is
// out of range. A zero value is "unset" and is skipped.
func appendFloatViolation(vs []Violation, field string, declared, ceiling float64, override bool) []Violation {
	if declared == 0 {
		return vs
	}
	if declared < 0 {
		return append(vs, Violation{Field: field, Msg: nonPositiveMsg(field)})
	}
	if !override && declared > ceiling {
		return append(vs, Violation{Field: field, Msg: aboveCeilingMsgFloat(field, declared, ceiling)})
	}
	return vs
}

// appendIntViolation checks one int cap field and appends a Violation if it is out of
// range. A zero value is "unset" and is skipped.
func appendIntViolation(vs []Violation, field string, declared, ceiling int, override bool) []Violation {
	if declared == 0 {
		return vs
	}
	if declared < 0 {
		return append(vs, Violation{Field: field, Msg: nonPositiveMsg(field)})
	}
	if !override && declared > ceiling {
		return append(vs, Violation{Field: field, Msg: aboveCeilingMsgInt(field, int64(declared), int64(ceiling))})
	}
	return vs
}

// appendInt64Violation checks one int64 cap field and appends a Violation if it is out
// of range. A zero value is "unset" and is skipped.
func appendInt64Violation(vs []Violation, field string, declared, ceiling int64, override bool) []Violation {
	if declared == 0 {
		return vs
	}
	if declared < 0 {
		return append(vs, Violation{Field: field, Msg: nonPositiveMsg(field)})
	}
	if !override && declared > ceiling {
		return append(vs, Violation{Field: field, Msg: aboveCeilingMsgInt(field, declared, ceiling)})
	}
	return vs
}

func nonPositiveMsg(field string) string {
	return fmt.Sprintf("%s must be greater than 0", field)
}

func aboveCeilingMsgInt(field string, declared, ceiling int64) string {
	return fmt.Sprintf("%s %d exceeds the ceiling of %d (use the override flag to raise it)", field, declared, ceiling)
}

func aboveCeilingMsgFloat(field string, declared, ceiling float64) string {
	return fmt.Sprintf("%s %g exceeds the ceiling of %g (use the override flag to raise it)", field, declared, ceiling)
}
