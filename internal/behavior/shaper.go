package behavior

import (
	"time"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// Shaper computes the wait BEFORE an action. It is Cause-aware: a uniform human
// pause before EVERY action produces a metronomic waterfall that fingerprints a
// bot, so only navigations get the full pipeline (design §4).
type Shaper interface {
	Shape(ShapeCtx) ShapeDecision
}

// ShapeCtx is the per-step input. BaseThink is sampled from the persona's
// ThinkTime by the Session; Prior is the previous action's scrubbed Result (nil
// on the first step) for closed-loop reactive think-time.
type ShapeCtx struct {
	Now       time.Time
	StepIndex int
	BaseThink time.Duration
	Cause     protocols.Cause
	Prior     *protocols.Result
	Rand      rng.Rand
}

// ShapeDecision is the wait the engine sleeps. Idle marks a burst trough (the
// Session emits an action-less idle Step).
type ShapeDecision struct {
	Wait time.Duration
	Idle bool
}

// subResourceJitterMax bounds the micro-jitter for browser-scale sub-resource
// fetches (CSS/JS/img bursts, A+AAAA fan-out) — no human pause, just a few ms.
const subResourceJitterMax = 15 * time.Millisecond

// Closed-loop reaction constants: a slow or failed prior action lengthens the
// next human pause by a bounded factor.
const (
	priorSlowThreshold = 2 * time.Second
	priorSlowFactor    = 1.5
	priorFailFactor    = 2.0
)

// chainShaper applies the fixed stage order think-time -> jitter -> burstiness ->
// time-of-day, branched by Cause.
type chainShaper struct {
	jitter JitterModel
	burst  BurstModel
	tod    TimeOfDayShaper
}

// NewChainShaper composes the three primitives. Nil components fall back to the
// no-op variants, so a persona may omit any of them.
func NewChainShaper(j JitterModel, b BurstModel, t TimeOfDayShaper) Shaper {
	if j == nil {
		j = NoJitter{}
	}
	if b == nil {
		b = AlwaysActive{}
	}
	if t == nil {
		t = FlatTimeOfDay{}
	}
	return &chainShaper{jitter: j, burst: b, tod: t}
}

func (c *chainShaper) Shape(ctx ShapeCtx) ShapeDecision {
	switch ctx.Cause {
	case protocols.CauseControl:
		return ShapeDecision{Wait: 0} // handshake/open/close: no human gap
	case protocols.CauseSubResource:
		w := time.Duration(ctx.Rand.Float64() * float64(subResourceJitterMax))
		return ShapeDecision{Wait: clampNonNeg(w)}
	}
	// CauseNavigation (and CauseBackground, which the Session paces separately but
	// shapes the same way): the full human pipeline.
	wait := c.jitter.Jitter(ctx.BaseThink, ctx.Rand) // think-time -> jitter
	if ph := c.burst.Phase(ctx.Now, ctx.Rand); ph.Idle {
		return ShapeDecision{Wait: ph.IdleFor, Idle: true} // burst trough
	}
	if intensity := c.tod.Intensity(ctx.Now); intensity > 0 {
		wait = time.Duration(float64(wait) / intensity) // quiet hours stretch waits
	}
	wait = applyPriorReaction(wait, ctx.Prior) // closed-loop reactive think-time
	return ShapeDecision{Wait: clampNonNeg(wait)}
}

// applyPriorReaction lengthens the next pause when the previous action was slow
// or failed (a human waits/retries more deliberately after an error).
func applyPriorReaction(wait time.Duration, prior *protocols.Result) time.Duration {
	if prior == nil {
		return wait
	}
	switch prior.Outcome {
	case protocols.OutcomeFailure, protocols.OutcomePanicked:
		return time.Duration(float64(wait) * priorFailFactor)
	}
	if prior.Latency >= priorSlowThreshold {
		return time.Duration(float64(wait) * priorSlowFactor)
	}
	return wait
}
