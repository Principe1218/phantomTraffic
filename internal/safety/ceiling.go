package safety

import "time"

// Ceiling is the ABSOLUTE, fleet-wide upper bound on how aggressively a
// PhantomTraffic run may behave. It is the config-time counterpart to the
// runtime per-session safety.Caps stub in caps.go: Ceiling expresses the maximum
// a scenario's declared caps are allowed to reach (design §6 — caps may be lowered
// freely by config but never raised above the ceiling except via the audited
// override flag). It is a plain value type with no methods that perform I/O.
//
// The first six fields are AGGREGATE caps shared across the whole agent fleet and
// are split per agent by DividedBy. The last two fields are PER-SESSION caps that
// bound a single virtual user and are therefore never divided.
type Ceiling struct {
	PerTargetRPS          float64       // aggregate max requests/sec to any one target
	GlobalRPS             float64       // aggregate max requests/sec across all targets
	MaxConcurrentSessions int           // aggregate max simultaneously-open sessions
	TotalRequestBudget    int64         // aggregate hard cap on total requests for the run
	StreamingByteRateKbps int           // aggregate streaming byte-rate cap (kilobits/sec)
	ConcurrentStreams     int           // aggregate max simultaneous media streams
	PerSessionMaxDuration time.Duration // per-session wall-clock lifetime cap
	PerSessionMaxActions  int           // per-session cap on total actions
}

// DefaultCeiling returns the D2 reference ceiling. These are the conservative,
// non-bypassable defaults; a scenario may declare lower caps but cannot exceed
// these without the audited override flag.
func DefaultCeiling() Ceiling {
	return Ceiling{
		PerTargetRPS:          10,
		GlobalRPS:             50,
		MaxConcurrentSessions: 20,
		TotalRequestBudget:    1_000_000,
		StreamingByteRateKbps: 12_000,
		ConcurrentStreams:     3,
		PerSessionMaxDuration: 30 * time.Minute,
		PerSessionMaxActions:  10_000,
	}
}

// DividedBy splits the AGGREGATE caps across agentCount agents so that the whole
// fleet, summed, stays within the original ceiling. Integer caps round down with a
// floor of 1 (no agent may receive a zero ceiling, which would deadlock it);
// float caps round naturally. The two PER-SESSION caps (PerSessionMaxDuration,
// PerSessionMaxActions) bound a single session and are returned unchanged.
//
// agentCount < 1 is defensively treated as 1 (the CLI guarantees a default of 1,
// but the function does not trust its caller — AGENTS.md §5.2 input validation).
// DividedBy has a value receiver and never mutates the original Ceiling.
func (c Ceiling) DividedBy(agentCount int) Ceiling {
	if agentCount < 1 {
		agentCount = 1
	}
	n := int64(agentCount)
	c.PerTargetRPS = c.PerTargetRPS / float64(agentCount)
	c.GlobalRPS = c.GlobalRPS / float64(agentCount)
	c.MaxConcurrentSessions = floorAtOne(c.MaxConcurrentSessions / agentCount)
	c.TotalRequestBudget = floorAtOne64(c.TotalRequestBudget / n)
	c.StreamingByteRateKbps = floorAtOne(c.StreamingByteRateKbps / agentCount)
	c.ConcurrentStreams = floorAtOne(c.ConcurrentStreams / agentCount)
	// PerSessionMaxDuration and PerSessionMaxActions are per-session: untouched.
	return c
}

// floorAtOne clamps an integer cap to a minimum of 1 so per-agent division never
// yields a zero (deadlocking) ceiling.
func floorAtOne(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

// floorAtOne64 is floorAtOne for the int64 TotalRequestBudget cap.
func floorAtOne64(v int64) int64 {
	if v < 1 {
		return 1
	}
	return v
}
