package behavior

import (
	"context"
	"fmt"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// Step is one decision from the Session: a wait the engine sleeps, plus an
// optional action. A nil Action with Done=false is a pure idle step (a burst
// trough or a benign no-target skip). The engine performs ALL sleeping; the
// Session never blocks (design §4).
type Step struct {
	Wait   time.Duration
	Action *PlannedAction
	Done   bool                 // session complete -> engine recycles the vuser
	Pacing protocols.PacingMode // SelfPaced -> engine honors Wait verbatim
}

// PlannedAction is a protocol-agnostic plan: a Ref (protocol+verb), an opaque
// Params (nil in Plan 3), the Cause, an optional Nav grouping, and a Target drawn
// from the allowlist by construction. The behavior layer never reads Params'
// fields, so credentials/bodies cannot leak upward (design §3).
type PlannedAction struct {
	Ref    protocols.Ref
	Params protocols.Params // nil in Plan 3 (concrete params land with handler plans)
	Cause  protocols.Cause
	Nav    protocols.NavID
	Target protocols.Target
	Label  string // stats correlation; never a secret
}

// Session is the pure, multi-protocol vuser state machine. Next returns the next
// Step; Observe feeds the prior Result back for closed-loop reactive think-time
// (open-loop in Plan 3 — Observation-driven branching is inert until handlers
// emit real Observations). A Session is single-goroutine; it is NOT safe for
// concurrent use.
type Session interface {
	Next(ctx context.Context) (Step, error)
	Observe(res protocols.Result, obs protocols.Observation)
	// Fingerprint is the ONE HTTP identity chosen at construction (zero value if
	// the persona has no FingerprintPool); HTTP handler plans thread it through
	// every request. The engine never mutates it.
	Fingerprint() Fingerprint
	// Bounds are the stateful-branching caps the handler plans enforce on
	// Observation-driven branches (redirect/CNAME/link/ABR).
	Bounds() BranchBounds
}

// SessionMaker builds Sessions from a SessionSpec + injected deps.
type SessionMaker interface {
	NewSession(ctx context.Context, spec SessionSpec, deps protocols.SessionDeps) (Session, error)
}

// SessionSpec is the decomposed persona bundle the sessionMaker needs (NOT a
// persona.Persona — that would create a behavior<->persona import cycle).
// internal/persona builds this via Persona.ToSpec.
type SessionSpec struct {
	Mix       TemplateMix
	ThinkTime Distribution
	Jitter    JitterModel
	Burst     BurstModel
	TimeOfDay TimeOfDayShaper
	Prints    FingerprintPool
	Shape     SessionShape
	Bounds    BranchBounds
	Selector  TargetSelector
}

type sessionMaker struct{}

// NewSessionMaker returns the default SessionMaker.
func NewSessionMaker() SessionMaker { return sessionMaker{} }

func (sessionMaker) NewSession(_ context.Context, spec SessionSpec, deps protocols.SessionDeps) (Session, error) {
	if spec.Mix.Len() == 0 {
		return nil, fmt.Errorf("behavior: SessionSpec.Mix must be non-empty")
	}
	if spec.Selector == nil {
		return nil, fmt.Errorf("behavior: SessionSpec.Selector is required")
	}
	if deps.Clock == nil || deps.Rand == nil {
		return nil, fmt.Errorf("behavior: SessionDeps.Clock and Rand are required")
	}
	r := deps.Rand.Split() // per-vuser stream derivation (design §4)
	think := spec.ThinkTime
	if think == nil {
		think = Constant{} // zero think-time fallback
	}
	var maxLen time.Duration
	if spec.Shape.Length != nil {
		maxLen = spec.Shape.Length.Sample(r) // sampled ONCE at construction
	}
	var fingerprint Fingerprint
	if spec.Prints != nil {
		fingerprint = spec.Prints.Pick(r) // ONE fingerprint for the whole session
	}
	return &session{
		mix:         spec.Mix,
		think:       think,
		shaper:      NewChainShaper(spec.Jitter, spec.Burst, spec.TimeOfDay),
		shape:       spec.Shape,
		bounds:      spec.Bounds,
		sel:         spec.Selector,
		deps:        deps,
		rand:        r,
		fingerprint: fingerprint,
		start:       deps.Clock.Now(),
		maxLen:      maxLen,
	}, nil
}

// session is the concrete, single-goroutine state machine.
type session struct {
	mix         TemplateMix
	think       Distribution
	shaper      Shaper
	shape       SessionShape
	bounds      BranchBounds
	sel         TargetSelector
	deps        protocols.SessionDeps
	rand        rng.Rand
	fingerprint Fingerprint
	start       time.Time
	maxLen      time.Duration
	step        int
	last        *protocols.Result // closed-loop Prior for reactive think-time
}

// Next computes the next Step. Draw order (fixed for determinism): terminate-by-length
// check -> abandon (only if Abandon>0) -> mix.Pick -> think.Sample -> shaper draws.
func (s *session) Next(ctx context.Context) (Step, error) {
	if err := ctx.Err(); err != nil {
		return Step{}, err
	}
	now := s.deps.Clock.Now()
	if s.maxLen > 0 && now.Sub(s.start) >= s.maxLen {
		return Step{Done: true}, nil // session length elapsed
	}
	if s.shape.Abandon > 0 && s.rand.Float64() < s.shape.Abandon {
		return Step{Done: true}, nil // human abandoned the session
	}

	tmpl := s.mix.Pick(s.rand)
	target, ok := s.sel.Next(tmpl.Protocol)
	if !ok {
		s.step++
		return Step{Wait: 0}, nil // benign skip: persona weighted an untargeted protocol
	}

	dec := s.shaper.Shape(ShapeCtx{
		Now:       now,
		StepIndex: s.step,
		Think:     func() time.Duration { return s.think.Sample(s.rand) },
		Cause:     tmpl.Cause,
		Prior:     s.last,
		Rand:      s.rand,
	})
	s.step++
	if dec.Idle {
		return Step{Wait: dec.Wait}, nil // burst trough: no action
	}
	return Step{
		Wait:   dec.Wait,
		Pacing: tmpl.Pacing,
		Action: &PlannedAction{
			Ref:    tmpl.Ref(),
			Cause:  tmpl.Cause,
			Target: target,
			Label:  tmpl.Ref().String(),
		},
	}, nil
}

// Observe stores the scrubbed Result so the NEXT Next() uses it as ShapeCtx.Prior
// (closed-loop reactive think-time). Observation-driven branching (ABR/CSRF/
// follow-link) is wired by the handler plans that emit those Observations; the
// BranchBounds primitive is carried on the session for them (open-loop in Plan 3).
func (s *session) Observe(res protocols.Result, _ protocols.Observation) {
	r := res
	s.last = &r
}

// Fingerprint returns the session's chosen HTTP identity (zero value if the
// persona has no FingerprintPool). HTTP handler plans thread it through requests.
func (s *session) Fingerprint() Fingerprint { return s.fingerprint }

// Bounds returns the stateful-branching caps the handler plans enforce.
func (s *session) Bounds() BranchBounds { return s.bounds }
