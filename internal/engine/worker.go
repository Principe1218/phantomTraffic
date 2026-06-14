package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/rng"
	"github.com/Principe1218/phantomTraffic/internal/safety"
)

// workerDeps bundles everything one worker goroutine needs. It is assembled once
// per vuser by the supervisor (Module E5) and is read-only for the worker's life.
type workerDeps struct {
	Clock    clock.Clock
	Rand     rng.Rand
	Log      *slog.Logger
	Registry *protocols.Registry
	Limiter  safety.Limiter
	Sem      *semaphore
	Stats    *collector
	Gate     *gate
	Breakers map[string]*safety.Breaker // by Target.ID
	Tripwire *safety.Tripwire

	MaxRetries  int
	BackoffBase time.Duration
	BackoffMax  time.Duration

	PanicStorm        int // per-worker panic count -> ClassSafety quarantine
	SessionMaxActions int // per-session backstop (actions)
}

// runWorker drives one vuser's behavior.Session to completion (design §6.2):
//
//	for {
//	  step := vs.Next; if Done/err -> return
//	  gate.wait (block while any pause source active)
//	  clock.Sleep(step.Wait)
//	  if step.Action != nil: runAction; stats.Record; vs.Observe
//	}
//
// It is single-goroutine: the Session is never touched concurrently. All blocking
// points are ctx-first so STOP unblocks them.
func runWorker(ctx context.Context, sess *protocols.Session, vs behavior.Session, d workerDeps) {
	actions := 0
	for {
		step, err := vs.Next(ctx)
		if err != nil || step.Done {
			return
		}
		if actions >= d.SessionMaxActions {
			return // per-session backstop (design §4.4)
		}

		if err := d.Gate.wait(ctx); err != nil {
			return // ctx canceled while paused
		}
		if err := d.Clock.Sleep(ctx, step.Wait); err != nil {
			return // ctx canceled during the inter-action wait
		}

		if step.Action == nil {
			continue // idle step (burst trough / benign skip)
		}

		res, obs, stop := runAction(ctx, sess, step.Action, d)
		if stop {
			return // ClassSafety tripped the run (latched)
		}
		actions++
		d.Stats.Record(res)
		vs.Observe(res, obs)
	}
}

// runAction performs one planned action through the full safety + routing +
// resilience composition (design §6.2 / §7.1). The bool return is "stop": true
// means the latched tripwire fired (ClassSafety) and the worker must return.
func runAction(ctx context.Context, sess *protocols.Session, pa *behavior.PlannedAction, d workerDeps) (protocols.Result, protocols.Observation, bool) {
	targetID := pa.Target.ID
	breaker := d.Breakers[targetID]

	// Open breaker => skip this step so siblings keep running (design §6.5/§7.1).
	if breaker != nil && !breaker.Allow() {
		return skipResult(pa), protocols.Observation{}, false
	}

	// Two-tier limiter admission. A ClassSafety error means a hard cap breach or
	// a tripped tripwire: latch and stop (highest precedence, no auto-reset).
	reservation, err := d.Limiter.Acquire(ctx, targetID, 0)
	if err != nil {
		if pterr.IsClass(err, pterr.ClassSafety) {
			d.Tripwire.Trip("limiter ClassSafety")
			return safetyResult(pa), protocols.Observation{}, true
		}
		return cancelResult(pa), protocols.Observation{}, false
	}
	defer reservation.Release()

	if err := d.Sem.acquire(ctx); err != nil {
		return cancelResult(pa), protocols.Observation{}, false
	}
	defer d.Sem.release()

	handler, ok := d.Registry.Lookup(pa.Ref.Protocol)
	if !ok {
		res := failureResult(pa, pterr.ClassPermanent, "routing.unknown")
		recordBreaker(breaker, res.Outcome)
		return res, protocols.Observation{}, false
	}

	res, obs := doWithRetry(ctx, sess, handler, pa, d)
	reservation.Reconcile(res.BytesIn + res.BytesOut)
	recordBreaker(breaker, res.Outcome)
	return res, obs, false
}

// doWithRetry invokes the handler inside the panic shim and applies the error
// taxonomy: ClassTransient (and ClassUnknown) retry with exponential backoff up to
// MaxRetries; everything else returns on the first attempt. A panic becomes an
// OutcomePanicked failure.
func doWithRetry(ctx context.Context, sess *protocols.Session, handler protocols.ProtocolHandler, pa *behavior.PlannedAction, d workerDeps) (protocols.Result, protocols.Observation) {
	var (
		res protocols.Result
		obs protocols.Observation
		err error
	)
	for attempt := 0; ; attempt++ {
		panicked := runGuarded(d.Log, func() {
			res, obs, err = handler.Do(ctx, sess, pa.Params)
		})
		if panicked {
			return panicResult(pa), protocols.Observation{}
		}

		class := classify(res, err)
		if err == nil && res.Outcome == protocols.OutcomeSuccess {
			return res, obs
		}

		retryable := class == pterr.ClassTransient || class == pterr.ClassUnknown
		if !retryable || attempt >= d.MaxRetries {
			return failedResult(pa, res, class), obs
		}

		delay := backoffDelay(attempt, d.BackoffBase, d.BackoffMax, d.Rand)
		if serr := d.Clock.Sleep(ctx, delay); serr != nil {
			return cancelResult(pa), obs
		}
	}
}

// classify collapses a handler's (Result, error) to a single pterr.Class.
func classify(res protocols.Result, err error) pterr.Class {
	if err != nil {
		return pterr.Classify(err)
	}
	if res.Outcome != protocols.OutcomeSuccess {
		return res.ErrClass
	}
	return pterr.ClassUnknown
}

func recordBreaker(b *safety.Breaker, outcome protocols.Outcome) {
	if b == nil {
		return
	}
	if outcome == protocols.OutcomeSuccess {
		b.RecordSuccess()
		return
	}
	b.RecordFailure()
}

// ----- Result constructors -----

func baseResult(pa *behavior.PlannedAction) protocols.Result {
	return protocols.Result{
		Protocol: pa.Ref.Protocol,
		Action:   pa.Ref.Verb,
		Target:   pa.Target.ID,
	}
}

func skipResult(pa *behavior.PlannedAction) protocols.Result {
	r := baseResult(pa)
	r.Outcome = protocols.OutcomeSkipped
	return r
}

func cancelResult(pa *behavior.PlannedAction) protocols.Result {
	r := baseResult(pa)
	r.Outcome = protocols.OutcomeCancelled
	return r
}

func safetyResult(pa *behavior.PlannedAction) protocols.Result {
	r := baseResult(pa)
	r.Outcome = protocols.OutcomeFailure
	r.ErrClass = pterr.ClassSafety
	r.ErrCode = "safety.cap"
	return r
}

func panicResult(pa *behavior.PlannedAction) protocols.Result {
	r := baseResult(pa)
	r.Outcome = protocols.OutcomePanicked
	r.ErrClass = pterr.ClassUnknown
	r.ErrCode = "panic"
	return r
}

func failureResult(pa *behavior.PlannedAction, class pterr.Class, code string) protocols.Result {
	r := baseResult(pa)
	r.Outcome = protocols.OutcomeFailure
	r.ErrClass = class
	r.ErrCode = code
	return r
}

// failedResult preserves the handler's own Result fields while stamping the failure
// outcome/class, so handler-supplied bytes/metadata survive into stats.
func failedResult(pa *behavior.PlannedAction, res protocols.Result, class pterr.Class) protocols.Result {
	if res.Protocol == "" {
		res = baseResult(pa)
	}
	res.Outcome = protocols.OutcomeFailure
	if res.ErrClass == 0 {
		res.ErrClass = class
	}
	return res
}
