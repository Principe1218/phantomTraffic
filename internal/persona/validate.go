package persona

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
)

// FieldError is one persona-validation problem with a YAML field path.
type FieldError struct {
	Field string
	Msg   string
}

func (e FieldError) Error() string { return e.Field + ": " + e.Msg }

// Errors aggregates every FieldError so Compile reports all problems at once.
type Errors []FieldError

func (es Errors) Error() string {
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "; ")
}

type accumulator struct{ errs Errors }

func (a *accumulator) add(field, msg string) {
	a.errs = append(a.errs, FieldError{Field: field, Msg: msg})
}

func (a *accumulator) result() error {
	if len(a.errs) == 0 {
		return nil
	}
	return a.errs
}

// Compile validates a RawPersona and builds an immutable Persona, aggregating
// EVERY field error (returned as Errors) instead of failing on the first. It is
// pure: no clock, no rand, no network, no filesystem (the embedded fingerprint
// table is read-only data). The same function validates built-ins and custom
// personas, so a built-in cannot rot silently.
func Compile(raw RawPersona) (Persona, error) {
	var acc accumulator
	p := Persona{Name: raw.Name, Bounds: behavior.DefaultBranchBounds()}

	if strings.TrimSpace(raw.Name) == "" {
		acc.add("name", "must be non-empty")
	}
	if tt, err := parseDist(raw.ThinkTime); err != nil {
		acc.add("think_time", err.Error())
	} else {
		p.ThinkTime = tt
	}
	if j, err := parseJitter(raw.Jitter); err != nil {
		acc.add("jitter", err.Error())
	} else {
		p.Jitter = j
	}
	if b, err := parseBurst(raw.Burst); err != nil {
		acc.add("burst", err.Error())
	} else {
		p.Burst = b
	}
	if tod, err := parseCurve(raw.TimeOfDay); err != nil {
		acc.add("time_of_day", err.Error())
	} else {
		p.TimeOfDay = tod
	}

	p.Shape = compileShape(&acc, raw.Session)
	p.Mix = compileMix(&acc, raw.Mix)

	if fp, err := resolveFingerprints(raw.Fingerprints); err != nil {
		acc.add("fingerprints", err.Error())
	} else {
		p.Prints = fp
	}

	if err := acc.result(); err != nil {
		return Persona{}, err
	}
	return p, nil
}

// compileShape validates the session shape, appending field errors to acc.
func compileShape(acc *accumulator, rs RawShape) behavior.SessionShape {
	var sh behavior.SessionShape
	if rs.Length.Kind != "" { // length is optional (omitted -> no cap)
		if d, err := parseDist(rs.Length); err != nil {
			acc.add("session.length", err.Error())
		} else {
			sh.Length = d
		}
	}
	if rs.InterTask.Kind != "" {
		if d, err := parseDist(rs.InterTask); err != nil {
			acc.add("session.inter_task", err.Error())
		} else {
			sh.InterTask = d
		}
	}
	if rs.Abandon < 0 || rs.Abandon > 1 {
		acc.add("session.abandon", "must be in [0,1]")
	}
	sh.Abandon = rs.Abandon
	return sh
}

// compileMix validates the weighted mix and builds a behavior.TemplateMix.
func compileMix(acc *accumulator, rms []RawTemplate) behavior.TemplateMix {
	if len(rms) == 0 {
		acc.add("mix", "at least one template is required")
		return behavior.TemplateMix{}
	}
	tmpls := make([]behavior.Template, 0, len(rms))
	ok := true
	for i, rm := range rms {
		fp := "mix[" + strconv.Itoa(i) + "]"
		proto, pok := parseProtocol(rm.Protocol)
		if !pok {
			acc.add(fp+".protocol", "unknown protocol "+strconv.Quote(rm.Protocol)+" (allowed: dns, http, ssh, stream)")
			ok = false
		}
		cause, cok := parseCause(rm.Cause)
		if !cok {
			acc.add(fp+".cause", "unknown cause "+strconv.Quote(rm.Cause))
			ok = false
		}
		pacing, pacok := parsePacing(rm.Pacing)
		if !pacok {
			acc.add(fp+".pacing", "unknown pacing "+strconv.Quote(rm.Pacing))
			ok = false
		}
		if !validVerb(rm.Verb) {
			acc.add(fp+".verb", "must be non-empty and match [a-z0-9-]")
			ok = false
		}
		if rm.Weight <= 0 {
			acc.add(fp+".weight", "must be > 0")
			ok = false
		}
		tmpls = append(tmpls, behavior.Template{
			Protocol: proto, Verb: protocols.ActionKind(rm.Verb),
			Cause: cause, Pacing: pacing, Weight: rm.Weight,
		})
	}
	if !ok {
		return behavior.TemplateMix{}
	}
	mix, err := behavior.NewTemplateMix(tmpls)
	if err != nil {
		acc.add("mix", err.Error())
		return behavior.TemplateMix{}
	}
	return mix
}

// resolveFingerprints maps a pool name to a FingerprintPool. Only the embedded
// "default" pool exists in Plan 3.
func resolveFingerprints(name string) (behavior.FingerprintPool, error) {
	switch name {
	case "", "default":
		return behavior.DefaultFingerprintPool()
	default:
		return nil, fmt.Errorf("unknown fingerprint pool %q (only %q)", name, "default")
	}
}
