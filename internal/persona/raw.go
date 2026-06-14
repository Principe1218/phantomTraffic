package persona

// RawPersona is the on-disk YAML shape of a persona. It is decoded strictly
// (KnownFields(true)) so a typo'd key is rejected, then turned into a validated
// Persona by Compile. Durations are strings (e.g. "30m") parsed in decode.
type RawPersona struct {
	Name         string        `yaml:"name"`
	ThinkTime    RawDist       `yaml:"think_time"`
	Jitter       RawJitter     `yaml:"jitter"`
	Burst        RawBurst      `yaml:"burst"`
	TimeOfDay    RawCurve      `yaml:"time_of_day"`
	Session      RawShape      `yaml:"session"`
	Mix          []RawTemplate `yaml:"mix"`
	Fingerprints string        `yaml:"fingerprints"`
}

// RawDist is the tagged-union on-disk shape of a Distribution. Only the fields
// relevant to Kind are read; durations are strings parsed by parseDist.
type RawDist struct {
	Kind   string  `yaml:"kind"`
	D      string  `yaml:"d"`      // constant
	Min    string  `yaml:"min"`    // uniform / normal
	Max    string  `yaml:"max"`    // uniform / normal
	Mean   string  `yaml:"mean"`   // normal / exponential
	StdDev string  `yaml:"stddev"` // normal
	Mu     float64 `yaml:"mu"`     // lognormal
	Sigma  float64 `yaml:"sigma"`  // lognormal
	Scale  string  `yaml:"scale"`  // lognormal
}

// RawJitter is the on-disk shape of a JitterModel.
type RawJitter struct {
	Kind     string  `yaml:"kind"`     // "" | none | proportional
	Fraction float64 `yaml:"fraction"` // proportional
}

// RawBurst is the on-disk shape of a BurstModel (omitted -> always active).
type RawBurst struct {
	Active RawDist `yaml:"active"`
	Idle   RawDist `yaml:"idle"`
}

// RawCurve is the on-disk shape of a PiecewiseCurve (omitted -> flat).
type RawCurve struct {
	Location string    `yaml:"location"`
	Weekday  []float64 `yaml:"weekday"`
	Weekend  []float64 `yaml:"weekend"`
}

// RawShape is the on-disk shape of a SessionShape.
type RawShape struct {
	Length    RawDist `yaml:"length"`
	InterTask RawDist `yaml:"inter_task"`
	Abandon   float64 `yaml:"abandon"`
}

// RawTemplate is the on-disk shape of one weighted mix entry.
type RawTemplate struct {
	Protocol string  `yaml:"protocol"`
	Verb     string  `yaml:"verb"`
	Cause    string  `yaml:"cause"`
	Pacing   string  `yaml:"pacing"`
	Weight   float64 `yaml:"weight"`
}
