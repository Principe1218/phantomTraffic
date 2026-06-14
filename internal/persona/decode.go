package persona

import (
	"fmt"
	"time"
	_ "time/tzdata" // embed the zone DB so LoadLocation resolves identically everywhere (stdlib, no new module)

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
)

// parseDur parses a required duration string (e.g. "30m"); empty is an error.
func parseDur(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("missing duration")
	}
	return time.ParseDuration(s)
}

// parseDist builds a behavior.Distribution from its tagged-union raw shape. It
// dispatches by kind to a per-kind builder; each builder parses its durations
// and validates its own parameter invariants.
func parseDist(rd RawDist) (behavior.Distribution, error) {
	switch rd.Kind {
	case "constant":
		return parseConstant(rd)
	case "uniform":
		return parseUniform(rd)
	case "normal":
		return parseNormal(rd)
	case "lognormal":
		return parseLogNormal(rd)
	case "exponential":
		return parseExponential(rd)
	case "":
		return nil, fmt.Errorf("kind is required")
	default:
		return nil, fmt.Errorf("unknown kind %q", rd.Kind)
	}
}

func parseConstant(rd RawDist) (behavior.Distribution, error) {
	d, err := parseDur(rd.D)
	if err != nil {
		return nil, fmt.Errorf("d: %w", err)
	}
	if d < 0 {
		return nil, fmt.Errorf("d must be >= 0")
	}
	return behavior.Constant{D: d}, nil
}

func parseUniform(rd RawDist) (behavior.Distribution, error) {
	mn, err := parseDur(rd.Min)
	if err != nil {
		return nil, fmt.Errorf("min: %w", err)
	}
	mx, err := parseDur(rd.Max)
	if err != nil {
		return nil, fmt.Errorf("max: %w", err)
	}
	if mn < 0 {
		return nil, fmt.Errorf("min must be >= 0")
	}
	if mx < mn {
		return nil, fmt.Errorf("max must be >= min")
	}
	return behavior.Uniform{Min: mn, Max: mx}, nil
}

func parseNormal(rd RawDist) (behavior.Distribution, error) {
	mean, err := parseDur(rd.Mean)
	if err != nil {
		return nil, fmt.Errorf("mean: %w", err)
	}
	std, err := parseDur(rd.StdDev)
	if err != nil {
		return nil, fmt.Errorf("stddev: %w", err)
	}
	mn, err := parseDur(rd.Min)
	if err != nil {
		return nil, fmt.Errorf("min: %w", err)
	}
	mx, err := parseDur(rd.Max)
	if err != nil {
		return nil, fmt.Errorf("max: %w", err)
	}
	if mean < 0 {
		return nil, fmt.Errorf("mean must be >= 0")
	}
	if std < 0 {
		return nil, fmt.Errorf("stddev must be >= 0")
	}
	if mn < 0 {
		return nil, fmt.Errorf("min must be >= 0")
	}
	if mx < mn {
		return nil, fmt.Errorf("max must be >= min")
	}
	return behavior.Normal{Mean: mean, StdDev: std, Min: mn, Max: mx}, nil
}

func parseLogNormal(rd RawDist) (behavior.Distribution, error) {
	scale, err := parseDur(rd.Scale)
	if err != nil {
		return nil, fmt.Errorf("scale: %w", err)
	}
	if rd.Sigma < 0 {
		return nil, fmt.Errorf("sigma must be >= 0")
	}
	if scale < 0 {
		return nil, fmt.Errorf("scale must be >= 0")
	}
	return behavior.LogNormal{Mu: rd.Mu, Sigma: rd.Sigma, Scale: scale}, nil
}

func parseExponential(rd RawDist) (behavior.Distribution, error) {
	mean, err := parseDur(rd.Mean)
	if err != nil {
		return nil, fmt.Errorf("mean: %w", err)
	}
	if mean <= 0 {
		return nil, fmt.Errorf("mean must be > 0")
	}
	return behavior.Exponential{Mean: mean}, nil
}

// parseJitter builds a behavior.JitterModel (omitted/none -> NoJitter).
func parseJitter(rj RawJitter) (behavior.JitterModel, error) {
	switch rj.Kind {
	case "", "none":
		return behavior.NoJitter{}, nil
	case "proportional":
		if rj.Fraction < 0 || rj.Fraction > 1 {
			return nil, fmt.Errorf("fraction must be in [0,1]")
		}
		return behavior.ProportionalJitter{Fraction: rj.Fraction}, nil
	default:
		return nil, fmt.Errorf("unknown jitter kind %q", rj.Kind)
	}
}

// parseBurst builds a behavior.BurstModel (omitted -> AlwaysActive).
func parseBurst(rb RawBurst) (behavior.BurstModel, error) {
	if rb.Active.Kind == "" && rb.Idle.Kind == "" {
		return behavior.AlwaysActive{}, nil
	}
	active, err := parseDist(rb.Active)
	if err != nil {
		return nil, fmt.Errorf("active: %w", err)
	}
	idle, err := parseDist(rb.Idle)
	if err != nil {
		return nil, fmt.Errorf("idle: %w", err)
	}
	return behavior.NewRenewalBurst(active, idle), nil
}

// parseCurve builds a behavior.TimeOfDayShaper (omitted -> FlatTimeOfDay).
func parseCurve(rc RawCurve) (behavior.TimeOfDayShaper, error) {
	if rc.Location == "" && len(rc.Weekday) == 0 && len(rc.Weekend) == 0 {
		return behavior.FlatTimeOfDay{}, nil
	}
	if rc.Location == "" {
		return nil, fmt.Errorf("location is required when weekday or weekend curves are provided")
	}
	loc, err := time.LoadLocation(rc.Location)
	if err != nil {
		return nil, fmt.Errorf("location: %w", err)
	}
	if len(rc.Weekday) != 24 {
		return nil, fmt.Errorf("weekday must have 24 entries, got %d", len(rc.Weekday))
	}
	if len(rc.Weekend) != 24 {
		return nil, fmt.Errorf("weekend must have 24 entries, got %d", len(rc.Weekend))
	}
	for i, v := range rc.Weekday {
		if v < 0 || v > 1 {
			return nil, fmt.Errorf("weekday[%d] must be in [0,1]", i)
		}
	}
	for i, v := range rc.Weekend {
		if v < 0 || v > 1 {
			return nil, fmt.Errorf("weekend[%d] must be in [0,1]", i)
		}
	}
	var wd, we [24]float64
	copy(wd[:], rc.Weekday)
	copy(we[:], rc.Weekend)
	return behavior.PiecewiseCurve{Loc: loc, Weekday: wd, Weekend: we}, nil
}

// parseProtocol maps a YAML protocol string to a canonical ProtocolID.
func parseProtocol(s string) (protocols.ProtocolID, bool) {
	p := protocols.ProtocolID(s)
	return p, protocols.IsKnownProtocol(p)
}

// parseCause maps a YAML cause string ("" -> navigation).
func parseCause(s string) (protocols.Cause, bool) {
	switch s {
	case "", "navigation":
		return protocols.CauseNavigation, true
	case "sub-resource":
		return protocols.CauseSubResource, true
	case "background":
		return protocols.CauseBackground, true
	case "control":
		return protocols.CauseControl, true
	default:
		return 0, false
	}
}

// parsePacing maps a YAML pacing string ("" -> shaper-managed).
func parsePacing(s string) (protocols.PacingMode, bool) {
	switch s {
	case "", "shaper-managed":
		return protocols.PacingShaperManaged, true
	case "self-paced":
		return protocols.PacingSelfPaced, true
	default:
		return 0, false
	}
}

// validVerb allows a low-cardinality, injection-safe verb charset (a verb becomes
// a Ref label): lowercase letters, digits, hyphen. No CRLF can survive this.
func validVerb(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
		default:
			return false
		}
	}
	return true
}
