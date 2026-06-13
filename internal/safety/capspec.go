package safety

// CapSpec is a DECLARED cap set — the config-time mirror of the YAML caps: block.
// A ZERO field means "unset": Effective replaces it with the corresponding ceiling
// value. A non-zero field is an explicit declaration the operator chose. Unlike the
// runtime safety.Caps stub in caps.go (which the engine consumes at run time), CapSpec
// is a pure config value with no behavior beyond Effective and the validation in
// ValidateCaps.
//
// Field note: PerSessionMaxDurationSeconds is an int count of SECONDS (straight from
// YAML per_session_max_duration_seconds), whereas Ceiling.PerSessionMaxDuration is a
// time.Duration. Effective bridges the two by expressing the ceiling duration in whole
// seconds when the field is unset.
type CapSpec struct {
	PerTargetRPS                 float64
	GlobalRPS                    float64
	MaxConcurrentSessions        int
	TotalRequestBudget           int64
	StreamingByteRateKbps        int
	ConcurrentStreams            int
	PerSessionMaxDurationSeconds int
	PerSessionMaxActions         int
}

// Effective resolves a declared CapSpec against a ceiling: every ZERO (unset) field
// is replaced by the ceiling value; every non-zero field is preserved. The result is
// the fully-resolved cap set the engine will enforce. Effective has a value receiver
// (named declared per the contract) and never mutates the original.
func (declared CapSpec) Effective(ceiling Ceiling) CapSpec {
	if declared.PerTargetRPS == 0 {
		declared.PerTargetRPS = ceiling.PerTargetRPS
	}
	if declared.GlobalRPS == 0 {
		declared.GlobalRPS = ceiling.GlobalRPS
	}
	if declared.MaxConcurrentSessions == 0 {
		declared.MaxConcurrentSessions = ceiling.MaxConcurrentSessions
	}
	if declared.TotalRequestBudget == 0 {
		declared.TotalRequestBudget = ceiling.TotalRequestBudget
	}
	if declared.StreamingByteRateKbps == 0 {
		declared.StreamingByteRateKbps = ceiling.StreamingByteRateKbps
	}
	if declared.ConcurrentStreams == 0 {
		declared.ConcurrentStreams = ceiling.ConcurrentStreams
	}
	if declared.PerSessionMaxDurationSeconds == 0 {
		declared.PerSessionMaxDurationSeconds = int(ceiling.PerSessionMaxDuration.Seconds())
	}
	if declared.PerSessionMaxActions == 0 {
		declared.PerSessionMaxActions = ceiling.PerSessionMaxActions
	}
	return declared
}
