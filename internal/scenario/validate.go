package scenario

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/persona"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
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

// errAccumulator gathers every pterr.FieldError in discovery order so Validate
// can report ALL problems in one pass instead of failing on the first.
type errAccumulator struct {
	errs pterr.FieldErrors
}

// add appends one field problem.
func (a *errAccumulator) add(field, msg string) {
	a.errs = append(a.errs, pterr.FieldError{Field: field, Msg: msg})
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
	if net.ParseIP(host) != nil {
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
		ConcurrentStreams:            rc.ConcurrentStreams,
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

	// Personas: built-ins overlaid with validated customs, resolved per block.
	personas := resolvePersonas(&acc, raw.Personas)

	// Effective caps must be known BEFORE the block loop: per-block concurrency is
	// bounded by the effective MaxConcurrentSessions (design §3.3). Compute the
	// ceiling + effective cap once, validate caps, then reuse the bound per block.
	ceiling := safety.DefaultCeiling().DividedBy(agentCount)
	declared := toCapSpec(raw.Caps)
	for _, v := range safety.ValidateCaps(declared, ceiling, opts.CapOverride) {
		acc.add("caps."+v.Field, v.Msg)
	}
	caps := declared.Effective(ceiling)
	maxConcurrent := caps.MaxConcurrentSessions

	// Blocks + targets. We collect every typed target across all blocks to build
	// the single frozen TargetSet.
	var allTargets []protocols.Target
	seenID := make(map[string]struct{}, len(raw.Scenarios))
	blocks := make([]Block, 0, len(raw.Scenarios))

	for i, rb := range raw.Scenarios {
		b := validateBlock(&acc, i, rb, opts, personas, maxConcurrent)
		if _, dup := seenID[rb.ID]; dup && rb.ID != "" {
			acc.add(fieldPath(i, "id"), "duplicate id "+strconv.Quote(rb.ID))
		}
		if rb.ID != "" {
			seenID[rb.ID] = struct{}{}
		}
		allTargets = append(allTargets, b.Targets...)
		blocks = append(blocks, b)
	}

	// allowed_domains are host-only allowlist entries (no port). Validate each so a
	// malformed entry can't silently become a dead, never-matching allowlist row
	// (design §5.2 — allowed_domains are host-validated, like targets).
	for i, d := range raw.AllowedDomains {
		if !validHost(strings.TrimSpace(d)) {
			acc.add("allowed_domains["+strconv.Itoa(i)+"]", "not a valid hostname")
		}
	}

	// Weighting strategy for the block mix (design §6.7). "" defaults to
	// WeightByVuserPopulation; an unknown value is rejected.
	weightBasis, wbOK := parseWeightBasis(raw.WeightBasis)
	if !wbOK {
		acc.add("weight_basis", "unknown weight_basis "+strconv.Quote(raw.WeightBasis)+" (allowed: vuser_population, concurrency, request_rate)")
	}

	schedule := validateSchedule(&acc, raw.Schedule)

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
		CapOverride:    opts.CapOverride,
		Execution:      Execution{Mode: mode, StopOnError: raw.Execution.StopOnError},
		Blocks:         blocks,
		Targets:        protocols.NewTargetSet(allTargets, raw.AllowedDomains),
		WeightBasis:    weightBasis,
		Schedule:       schedule,
	}, nil
}

// fieldPath renders the YAML path for the i-th scenario block's sub-key.
func fieldPath(i int, sub string) string {
	return "scenarios[" + strconv.Itoa(i) + "]." + sub
}

// resolvePersonas builds the name->Persona map the blocks resolve against:
// the embedded built-ins overlaid with any validated custom personas. Each
// custom persona is Compile-validated; its field errors are re-keyed under
// personas[i].<field> and aggregated. A custom persona overrides a built-in of
// the same name.
func resolvePersonas(acc *errAccumulator, raws []persona.RawPersona) map[string]persona.Persona {
	out := map[string]persona.Persona{}
	builtins, err := persona.Builtins()
	if err != nil {
		acc.add("personas", "internal: failed to load built-in personas: "+err.Error())
	} else {
		for name, p := range builtins {
			out[name] = p
		}
	}
	for i, rp := range raws {
		p, cerr := persona.Compile(rp)
		if cerr != nil {
			var pe pterr.FieldErrors
			if errors.As(cerr, &pe) {
				for _, fe := range pe {
					acc.add("personas["+strconv.Itoa(i)+"]."+fe.Field, fe.Msg)
				}
			} else {
				acc.add("personas["+strconv.Itoa(i)+"]", cerr.Error())
			}
			continue
		}
		out[p.Name] = p
	}
	return out
}

// validateBlock validates one RawBlock, appending any FieldErrors to acc, and
// returns the typed Block (partial if invalid; the accumulated errors prevent
// the partial result from ever being returned to the caller).
func validateBlock(acc *errAccumulator, i int, rb RawBlock, opts Options, personas map[string]persona.Persona, maxConcurrent int) Block {
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

	name := strings.TrimSpace(rb.Persona)
	if name == "" {
		name = persona.DefaultPersonaName // a block may omit persona:
	}
	if p, ok := personas[name]; ok {
		b.Persona = p
	} else {
		acc.add(fieldPath(i, "persona"), "unknown persona "+strconv.Quote(name))
	}

	b.Concurrency, b.Duration, b.Weight = validateBlockLoad(acc, i, rb, maxConcurrent)
	b.Ramp = validateRamp(acc, i, rb.Ramp, b.Concurrency, b.Duration)

	validateInsecureGate(acc, i, rb, opts)
	return b
}

// validateBlockLoad validates and builds the per-block load fields:
// concurrency (defaults to 1; in [1, maxConcurrent]), duration_minutes
// (required, >= 1), and weight (defaults to 1; >= 1). It appends ClassConfig
// FieldErrors to acc and returns the built values (partial on error — the
// accumulated errors prevent the partial result from reaching the caller).
func validateBlockLoad(acc *errAccumulator, i int, rb RawBlock, maxConcurrent int) (int, time.Duration, uint) {
	concurrency := rb.Concurrency
	if concurrency == 0 {
		concurrency = 1 // omitted concurrency defaults to a single vuser
	}
	if concurrency < 1 {
		acc.add(fieldPath(i, "concurrency"), "must be >= 1")
	} else if concurrency > maxConcurrent {
		acc.add(fieldPath(i, "concurrency"),
			"concurrency "+strconv.Itoa(concurrency)+" exceeds the effective max_concurrent_sessions cap of "+strconv.Itoa(maxConcurrent))
	}

	if rb.DurationMinutes < 1 {
		acc.add(fieldPath(i, "duration_minutes"), "must be >= 1")
	}
	duration := time.Duration(rb.DurationMinutes) * time.Minute

	weight := rb.Weight
	if weight == 0 {
		weight = 1 // omitted weight defaults to 1
	}

	return concurrency, duration, weight
}

// validateRamp validates and builds a block's RampPlan. A nil *RawRamp yields the
// zero RampPlan (no ramp). up_seconds must be in [0, duration]; start_concurrency
// defaults to concurrency (=> no ramp) and must be in [1, concurrency]. Errors are
// ClassConfig FieldErrors appended to acc. The concurrency/duration bounds come
// from the already-built block load fields (Task S4).
func validateRamp(acc *errAccumulator, i int, rr *RawRamp, concurrency int, duration time.Duration) RampPlan {
	if rr == nil {
		return RampPlan{} // no ramp
	}

	up := time.Duration(rr.UpSeconds) * time.Second
	if rr.UpSeconds < 0 {
		acc.add(fieldPath(i, "ramp.up_seconds"), "must be >= 0")
	} else if up > duration {
		acc.add(fieldPath(i, "ramp.up_seconds"), "up_seconds must be <= the block duration")
	}

	start := rr.StartConcurrency
	if start == 0 {
		start = concurrency // omitted start_concurrency => start at full concurrency (no ramp)
	}
	if start < 1 || start > concurrency {
		acc.add(fieldPath(i, "ramp.start_concurrency"),
			"start_concurrency must be in 1.."+strconv.Itoa(concurrency))
	}

	return RampPlan{Up: up, StartConcurrency: start}
}

// weekdayByName maps lowercase weekday abbreviations to time.Weekday for the
// schedule days allowlist (AGENTS.md §5.2: allowlist, not denylist).
var weekdayByName = map[string]time.Weekday{
	"sun": time.Sunday,
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
}

// validateSchedule validates and builds the scenario-level Schedule. A nil
// *RawSchedule yields the empty always-active Schedule (design §5). The timezone
// must load via time.LoadLocation; each window's days must be a non-empty subset
// of the weekday allowlist; start/end must parse as HH:MM with end > start
// (cross-midnight is rejected by that same check). Errors are ClassConfig.
func validateSchedule(acc *errAccumulator, rs *RawSchedule) Schedule {
	if rs == nil {
		return Schedule{} // always active
	}

	loc, err := time.LoadLocation(rs.Timezone)
	if err != nil {
		acc.add("schedule.timezone", "failed to load timezone "+strconv.Quote(rs.Timezone))
		loc = nil
	}

	windows := make([]ScheduleWindow, 0, len(rs.Windows))
	for i, rw := range rs.Windows {
		windows = append(windows, validateWindow(acc, i, rw))
	}
	return Schedule{Loc: loc, Windows: windows}
}

// validateWindow validates and builds one ScheduleWindow. It checks a non-empty
// day allowlist, HH:MM parsing of start/end, and end > start. Field paths are
// "schedule.windows[i].<sub>". Errors are ClassConfig appended to acc.
func validateWindow(acc *errAccumulator, i int, rw RawWindow) ScheduleWindow {
	var w ScheduleWindow

	if len(rw.Days) == 0 {
		acc.add(scheduleWindowPath(i, "days"), "at least one day is required")
	}
	for j, d := range rw.Days {
		wd, ok := weekdayByName[strings.ToLower(strings.TrimSpace(d))]
		if !ok {
			acc.add(scheduleWindowPath(i, "days")+"["+strconv.Itoa(j)+"]", "unknown day "+strconv.Quote(d))
			continue
		}
		w.Days[int(wd)] = true
	}

	start, startOK := parseHHMM(rw.Start)
	if !startOK {
		acc.add(scheduleWindowPath(i, "start"), "start must be HH:MM in 00:00..23:59")
	}
	end, endOK := parseHHMM(rw.End)
	if !endOK {
		acc.add(scheduleWindowPath(i, "end"), "end must be HH:MM in 00:00..23:59")
	}
	if startOK && endOK && end <= start {
		// A cross-midnight window (end <= start after wrapping) is rejected in the
		// MVP: split it into two windows (design §3.3).
		acc.add(scheduleWindowPath(i, "end"), "end must be after start (no cross-midnight windows)")
	}

	w.Start, w.End = start, end
	return w
}

// scheduleWindowPath renders the YAML path for the i-th schedule window's sub-key.
func scheduleWindowPath(i int, sub string) string {
	return "schedule.windows[" + strconv.Itoa(i) + "]." + sub
}

// parseHHMM parses an "HH:MM" 24-hour clock string into a since-midnight
// time.Duration. ok is false for any malformed value or an out-of-range
// hour (0..23) or minute (0..59).
func parseHHMM(s string) (time.Duration, bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, herr := strconv.Atoi(parts[0])
	m, merr := strconv.Atoi(parts[1])
	if herr != nil || merr != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, true
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
