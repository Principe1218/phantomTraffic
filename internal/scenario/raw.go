package scenario

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
}

// Raw is the whole decoded scenario file. There is intentionally NO agent_count
// field: agent count is a CLI flag only (see Module 5 Options.AgentCount).
type Raw struct {
	Name           string       `yaml:"name"`
	Description    string       `yaml:"description"`
	AllowedDomains []string     `yaml:"allowed_domains"`
	Caps           RawCaps      `yaml:"caps"`
	Execution      RawExecution `yaml:"execution"`
	Scenarios      []RawBlock   `yaml:"scenarios"`
}
