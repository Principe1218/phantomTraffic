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

// parseDist builds a behavior.Distribution from its tagged-union raw shape.
func parseDist(rd RawDist) (behavior.Distribution, error) {
	switch rd.Kind {
	case "constant":
		d, err := parseDur(rd.D)
		if err != nil {
			return nil, fmt.Errorf("d: %w", err)
		}
		return behavior.Constant{D: d}, nil
	case "uniform":
		mn, err := parseDur(rd.Min)
		if err != nil {
			return nil, fmt.Errorf("min: %w", err)
		}
		mx, err := parseDur(rd.Max)
		if err != nil {
			return nil, fmt.Errorf("max: %w", err)
		}
		return behavior.Uniform{Min: mn, Max: mx}, nil
	case "normal":
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
		return behavior.Normal{Mean: mean, StdDev: std, Min: mn, Max: mx}, nil
	case "lognormal":
		scale, err := parseDur(rd.Scale)
		if err != nil {
			return nil, fmt.Errorf("scale: %w", err)
		}
		return behavior.LogNormal{Mu: rd.Mu, Sigma: rd.Sigma, Scale: scale}, nil
	case "exponential":
		mean, err := parseDur(rd.Mean)
		if err != nil {
			return nil, fmt.Errorf("mean: %w", err)
		}
		return behavior.Exponential{Mean: mean}, nil
	case "":
		return nil, fmt.Errorf("kind is required")
	default:
		return nil, fmt.Errorf("unknown kind %q", rd.Kind)
	}
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
	var wd, we [24]float64
	for i := 0; i < 24; i++ {
		if rc.Weekday[i] < 0 || rc.Weekday[i] > 1 {
			return nil, fmt.Errorf("weekday[%d] must be in [0,1]", i)
		}
		if rc.Weekend[i] < 0 || rc.Weekend[i] > 1 {
			return nil, fmt.Errorf("weekend[%d] must be in [0,1]", i)
		}
		wd[i], we[i] = rc.Weekday[i], rc.Weekend[i]
	}
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
