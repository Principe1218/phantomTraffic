package scenario

import "github.com/Principe1218/phantomTraffic/internal/persona"

// RawCaps mirrors safety.CapSpec one-for-one as the on-disk YAML shape. Every
// field is a plain scalar with an explicit yaml tag; a ZERO value means "unset"
// (inherit the ceiling) and is interpreted by Validate (Module 5), not here.
type RawCaps struct {
	PerTargetRPS                 float64 `yaml:"per_target_rps"`
	GlobalRPS                    float64 `yaml:"global_rps"`
	MaxConcurrentSessions        int     `yaml:"max_concurrent_sessions"`
	TotalRequestBudget           int64   `yaml:"total_request_budget"`
	StreamingByteRateKbps        int     `yaml:"streaming_byte_rate_kbps"`
	ConcurrentStreams            int     `yaml:"concurrent_streams"`
	PerSessionMaxDurationSeconds int     `yaml:"per_session_max_duration_seconds"`
	PerSessionMaxActions         int     `yaml:"per_session_max_actions"`
}

// RawExecution is the on-disk shape of the run-orchestration block.
type RawExecution struct {
	Mode        string `yaml:"mode"`
	StopOnError bool   `yaml:"stop_on_error"`
}

// RawRamp is the on-disk shape of a per-block concurrency ramp. A nil *RawRamp on
// a RawBlock means "no ramp". Both fields are seconds/counts validated by Validate
// (Module S): up_seconds in 0..duration, start_concurrency in 1..concurrency.
type RawRamp struct {
	UpSeconds        int `yaml:"up_seconds"`
	StartConcurrency int `yaml:"start_concurrency"`
}

// RawWindow is the on-disk shape of one schedule window. Days are lowercase
// weekday abbreviations (mon..sun); Start/End are "HH:MM" strings parsed and
// bounds-checked (End > Start, no cross-midnight) by Validate (Module S).
type RawWindow struct {
	Days  []string `yaml:"days"`
	Start string   `yaml:"start"`
	End   string   `yaml:"end"`
}

// RawSchedule is the on-disk shape of the optional scenario-level schedule. A nil
// *RawSchedule on Raw means "always active" (no scheduler). Timezone loads via
// time.LoadLocation at Validate (Module S).
type RawSchedule struct {
	Timezone string      `yaml:"timezone"`
	Windows  []RawWindow `yaml:"windows"`
}

// RawBlock is the on-disk shape of one scenario block. Targets are raw
// "host[:port]" strings; they are parsed and validated by Validate (Module 5).
type RawBlock struct {
	ID                            string   `yaml:"id"`
	Protocol                      string   `yaml:"protocol"`
	Targets                       []string `yaml:"targets"`
	TargetRotation                string   `yaml:"target_rotation"`
	TargetRotationIntervalSeconds int      `yaml:"target_rotation_interval_seconds"`
	AllowInsecure                 bool     `yaml:"allow_insecure"`
	AllowInsecureReason           string   `yaml:"allow_insecure_reason"`
	Persona                       string   `yaml:"persona"`
	Concurrency                   int      `yaml:"concurrency"`
	DurationMinutes               int      `yaml:"duration_minutes"`
	Weight                        uint     `yaml:"weight"`
	Ramp                          *RawRamp `yaml:"ramp"`
}

// Raw is the whole decoded scenario file. There is intentionally NO agent_count
// field: agent count is a CLI flag only (see Module 5 Options.AgentCount).
type Raw struct {
	Name           string               `yaml:"name"`
	Description    string               `yaml:"description"`
	AllowedDomains []string             `yaml:"allowed_domains"`
	Caps           RawCaps              `yaml:"caps"`
	Execution      RawExecution         `yaml:"execution"`
	Scenarios      []RawBlock           `yaml:"scenarios"`
	Personas       []persona.RawPersona `yaml:"personas"`
	WeightBasis    string               `yaml:"weight_basis"`
	Schedule       *RawSchedule         `yaml:"schedule"`
}
