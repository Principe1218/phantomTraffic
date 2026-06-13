package scenario

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/safety"
)

// hostMaxLen / labelMaxLen bound a target hostname per RFC 1035 §2.3.4 /
// §3.1 (total <= 253 after trimming a trailing dot; each label <= 63).
const (
	hostMaxLen  = 253
	labelMaxLen = 63
)

// Options carries the invocation-time flags that gate validation. They come
// from CLI flags (cmd/phantom), never from the scenario file: a file alone can
// never relax a safety control (D3/D4). AgentCount < 1 is treated as 1.
type Options struct {
	AllowInsecure bool
	CapOverride   bool
	AgentCount    int
}

// errAccumulator gathers every FieldError in discovery order so Validate can
// report ALL problems in one pass instead of failing on the first.
type errAccumulator struct {
	errs ValidationErrors
}

// add appends one field problem.
func (a *errAccumulator) add(field, msg string) {
	a.errs = append(a.errs, FieldError{Field: field, Msg: msg})
}

// result returns the aggregated errors as a non-nil error if any exist, else
// nil. The empty-slice -> nil conversion is what makes a caller's `err != nil`
// check correct for a valid scenario.
func (a *errAccumulator) result() error {
	if len(a.errs) == 0 {
		return nil
	}
	return a.errs
}

// parseProtocol maps a YAML protocol string to a canonical ProtocolID using the
// compile-time known set (NOT the runtime registry, which is empty until
// handlers land). ok is false for an unknown value.
func parseProtocol(s string) (protocols.ProtocolID, bool) {
	p := protocols.ProtocolID(s)
	if protocols.IsKnownProtocol(p) {
		return p, true
	}
	return "", false
}

// parseRotation maps a YAML target_rotation string to a RotationStrategy.
// "" defaults to RotationSequential. ok is false for an unknown value.
func parseRotation(s string) (RotationStrategy, bool) {
	switch s {
	case "", "sequential":
		return RotationSequential, true
	case "random":
		return RotationRandom, true
	default:
		return RotationSequential, false
	}
}

// parseMode maps a YAML execution.mode string to an ExecutionMode.
// "" defaults to ExecParallel. ok is false for an unknown value.
func parseMode(s string) (ExecutionMode, bool) {
	switch s {
	case "", "parallel":
		return ExecParallel, true
	case "sequential":
		return ExecSequential, true
	default:
		return ExecParallel, false
	}
}

// validateTarget validates one "host[:port]" string and, on success, returns a
// typed protocols.Target with the zero CredentialRef. It rejects embedded URL
// credentials ('@'), URL schemes ("://"), invalid host charset/length, and
// ports outside 1..65535. On any violation it returns ok=false and a redacted
// reason; the caller attaches the field path. The credential is ALWAYS the zero
// value here — Plan 2 never reads or embeds credential material (AGENTS.md §3).
func validateTarget(addr string, proto protocols.ProtocolID) (protocols.Target, string, bool) {
	raw := strings.TrimSpace(addr)
	if raw == "" {
		return protocols.Target{}, "target must be non-empty", false
	}
	if strings.Contains(raw, "://") {
		return protocols.Target{}, "target must be host[:port], not a URL (remove the scheme)", false
	}
	if strings.Contains(raw, "@") {
		return protocols.Target{}, "target must not contain embedded credentials ('@')", false
	}

	host, hasPort, port := raw, false, ""
	// Split a trailing :port for IPv4/hostname forms only. Bracketed IPv6 keeps
	// its brackets; a bare IPv6 (multiple colons, no brackets) is rejected below
	// as an invalid host charset.
	if strings.HasPrefix(raw, "[") {
		h, p, err := net.SplitHostPort(raw)
		if err == nil {
			host, port, hasPort = h, p, true
		} else {
			host = strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")
		}
	} else if strings.Count(raw, ":") == 1 {
		i := strings.LastIndexByte(raw, ':')
		host, port, hasPort = raw[:i], raw[i+1:], true
	} else if strings.Count(raw, ":") > 1 {
		return protocols.Target{}, "target host is not a valid hostname or IP", false
	}

	if hasPort {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return protocols.Target{}, "port must be in 1..65535", false
		}
	}

	if !validHost(host) {
		return protocols.Target{}, "target host is not a valid hostname or IP", false
	}

	return protocols.Target{ID: raw, Proto: proto, Addr: raw}, "", true
}

// validHost reports whether host is a valid IP literal OR an RFC-1035-ish
// hostname (allowlist charset: letters/digits/hyphen/dot; each label 1..63,
// no leading/trailing hyphen; total <= 253). This is an allowlist, not a
// denylist (AGENTS.md §5.2).
func validHost(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return true
	}
	if len(host) > hostMaxLen {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if !validLabel(label) {
			return false
		}
	}
	return true
}

// validLabel reports whether one DNS label is allowlist-valid.
func validLabel(label string) bool {
	if label == "" || len(label) > labelMaxLen {
		return false
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for i := 0; i < len(label); i++ {
		c := label[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-':
		default:
			return false
		}
	}
	return true
}

// toCapSpec maps the YAML RawCaps onto safety.CapSpec. A zero field means
// "unset" -> inherit the ceiling (handled by safety.CapSpec.Effective).
func toCapSpec(rc RawCaps) safety.CapSpec {
	return safety.CapSpec{
		PerTargetRPS:                 rc.PerTargetRPS,
		GlobalRPS:                    rc.GlobalRPS,
		MaxConcurrentSessions:        rc.MaxConcurrentSessions,
		TotalRequestBudget:           rc.TotalRequestBudget,
		StreamingByteRateKbps:        rc.StreamingByteRateKbps,
		ConcurrentStreams:             rc.ConcurrentStreams,
		PerSessionMaxDurationSeconds: rc.PerSessionMaxDurationSeconds,
		PerSessionMaxActions:         rc.PerSessionMaxActions,
	}
}

// Validate is the PURE aggregator: it turns a decoded Raw into a FROZEN Scenario
// or a non-nil ValidationErrors listing EVERY problem found (each ClassConfig).
// It performs NO network, NO filesystem, NO runtime enforcement, NO audit write.
// Effective caps are declared.Effective(DefaultCeiling().DividedBy(agentCount));
// the (divided) ceiling is stored on the Scenario for later distributed
// enforcement (Plan 4). AgentCount < 1 is normalized to 1.
func Validate(raw Raw, opts Options) (Scenario, error) {
	var acc errAccumulator

	agentCount := opts.AgentCount
	if agentCount < 1 {
		agentCount = 1
	}

	if strings.TrimSpace(raw.Name) == "" {
		acc.add("name", "must be non-empty")
	}
	if len(raw.Scenarios) == 0 {
		acc.add("scenarios", "at least one scenario block is required")
	}

	// Execution mode.
	mode, ok := parseMode(raw.Execution.Mode)
	if !ok {
		acc.add("execution.mode", "unknown mode "+strconv.Quote(raw.Execution.Mode)+" (allowed: parallel, sequential)")
	}

	// Blocks + targets. We collect every typed target across all blocks to build
	// the single frozen TargetSet.
	var allTargets []protocols.Target
	seenID := make(map[string]struct{}, len(raw.Scenarios))
	blocks := make([]Block, 0, len(raw.Scenarios))

	for i, rb := range raw.Scenarios {
		b := validateBlock(&acc, i, rb, opts)
		if _, dup := seenID[rb.ID]; dup && rb.ID != "" {
			acc.add(fieldPath(i, "id"), "duplicate id "+strconv.Quote(rb.ID))
		}
		if rb.ID != "" {
			seenID[rb.ID] = struct{}{}
		}
		allTargets = append(allTargets, b.Targets...)
		blocks = append(blocks, b)
	}

	// Caps: map -> validate against the (divided) effective ceiling.
	ceiling := safety.DefaultCeiling().DividedBy(agentCount)
	declared := toCapSpec(raw.Caps)
	for _, v := range safety.ValidateCaps(declared, ceiling, opts.CapOverride) {
		acc.add("caps."+v.Field, v.Msg)
	}
	caps := declared.Effective(ceiling)

	if err := acc.result(); err != nil {
		return Scenario{}, err
	}

	return Scenario{
		Name:           raw.Name,
		Description:    raw.Description,
		AllowedDomains: raw.AllowedDomains,
		AgentCount:     agentCount,
		Caps:           caps,
		Ceiling:        ceiling,
		Execution:      Execution{Mode: mode, StopOnError: raw.Execution.StopOnError},
		Blocks:         blocks,
		Targets:        protocols.NewTargetSet(allTargets, raw.AllowedDomains),
	}, nil
}

// fieldPath renders the YAML path for the i-th scenario block's sub-key.
func fieldPath(i int, sub string) string {
	return "scenarios[" + strconv.Itoa(i) + "]." + sub
}

// validateBlock validates one RawBlock, appending any FieldErrors to acc, and
// returns the typed Block (partial if invalid; the accumulated errors prevent
// the partial result from ever being returned to the caller).
func validateBlock(acc *errAccumulator, i int, rb RawBlock, opts Options) Block {
	b := Block{
		ID:                  rb.ID,
		AllowInsecure:       rb.AllowInsecure,
		AllowInsecureReason: rb.AllowInsecureReason,
	}

	if strings.TrimSpace(rb.ID) == "" {
		acc.add(fieldPath(i, "id"), "must be non-empty")
	}

	proto, ok := parseProtocol(rb.Protocol)
	if !ok {
		acc.add(fieldPath(i, "protocol"), "unknown protocol "+strconv.Quote(rb.Protocol)+" (allowed: dns, http, ssh, stream)")
	}
	b.Protocol = proto

	if rot, rotOK := parseRotation(rb.TargetRotation); rotOK {
		b.Rotation = rot
	} else {
		acc.add(fieldPath(i, "target_rotation"), "unknown rotation "+strconv.Quote(rb.TargetRotation)+" (allowed: sequential, random)")
	}

	if rb.TargetRotationIntervalSeconds < 0 {
		acc.add(fieldPath(i, "target_rotation_interval_seconds"), "must be >= 0")
	} else {
		b.RotationInterval = time.Duration(rb.TargetRotationIntervalSeconds) * time.Second
	}

	if len(rb.Targets) == 0 {
		acc.add(fieldPath(i, "targets"), "at least one target is required")
	}
	for j, addr := range rb.Targets {
		tgt, reason, tok := validateTarget(addr, proto)
		if !tok {
			acc.add(fieldPath(i, "targets")+"["+strconv.Itoa(j)+"]", reason)
			continue
		}
		b.Targets = append(b.Targets, tgt)
	}

	validateInsecureGate(acc, i, rb, opts)
	return b
}

// validateInsecureGate enforces D4: a block may declare allow_insecure only with
// a non-empty reason AND only when the operator passed --allow-insecure. A
// scenario file alone can never unlock insecure transport.
func validateInsecureGate(acc *errAccumulator, i int, rb RawBlock, opts Options) {
	if !rb.AllowInsecure {
		return
	}
	if strings.TrimSpace(rb.AllowInsecureReason) == "" {
		acc.add(fieldPath(i, "allow_insecure_reason"), "must be non-empty when allow_insecure is true")
	}
	if !opts.AllowInsecure {
		acc.add(fieldPath(i, "allow_insecure"), "block "+rb.ID+": allow_insecure requires the --allow-insecure invocation flag")
	}
}
