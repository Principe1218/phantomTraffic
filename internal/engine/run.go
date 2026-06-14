package engine

import (
	"context"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/idgen"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
	"github.com/Principe1218/phantomTraffic/internal/safety"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
	"github.com/Principe1218/phantomTraffic/internal/schedule"
)

// Backstop tuning for the supervisor (design §6.6, §6.8).
const (
	breakerThreshold = 5
	breakerCooldown  = 30 * time.Second
)

// Start builds the cancellation tree and supervisor and returns immediately with
// the run in StateRunning (design §9 — Start is non-blocking).
func (e *Engine) Start(ctx context.Context, sc scenario.Scenario) (*Run, error) {
	const op = "engine.Start"
	runID, err := idgen.CorrelationID()
	if err != nil {
		return nil, pterr.Wrap(pterr.ClassPermanent, "engine.id", op, "mint run id", err)
	}

	runCtx, cancel := context.WithCancel(ctx)

	targetIDs := targetIDsOf(sc)
	tw := safety.NewTripwire(sc.Caps.TotalRequestBudget)
	lim := safety.NewLimiter(e.opts.Clock, sc.Caps, tw)
	breakers := make(map[string]*safety.Breaker, len(targetIDs))
	for _, id := range targetIDs {
		breakers[id] = safety.NewBreaker(e.opts.Clock, breakerThreshold, breakerCooldown)
	}
	coll := newCollector(targetIDs, e.opts.Clock, lim.Saturation)
	pub := newPublisher(coll.snapshot())

	r := &Run{
		id:          runID,
		agentID:     e.opts.AgentID,
		clk:         e.opts.Clock,
		audit:       e.opts.Audit,
		engine:      e,
		sc:          sc,
		runCtx:      runCtx,
		cancel:      cancel,
		gate:        newGate(e.opts.Clock),
		coll:        coll,
		pub:         pub,
		limiter:     lim,
		breakers:    breakers,
		tripwire:    tw,
		done:        make(chan struct{}),
		caps:        sc.Caps,
		capOverride: sc.CapOverride,
	}
	r.state.Store(int32(StateIdle))

	if err := r.transitionTo(StateRunning); err != nil {
		cancel()
		return nil, pterr.Wrap(pterr.ClassPermanent, "engine.state", op, "enter running", err)
	}

	r.startSupervisor()
	return r, nil
}

// startSupervisor spawns the supervisor goroutine tree. Each goroutine is tracked
// by r.wg so Stop()/completion can drain them.
func (r *Run) startSupervisor() {
	remaining := len(r.sc.Blocks)
	r.blocksLeft.Store(int32(remaining))

	r.weights = make(map[string]uint, len(r.sc.Blocks))
	for _, b := range r.sc.Blocks {
		r.weights[b.ID] = b.Weight
	}

	for i := range r.sc.Blocks {
		block := r.sc.Blocks[i]
		r.startBlock(block)
	}

	if len(r.sc.Schedule.Windows) > 0 {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.runScheduler()
		}()
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.runStatsCollector()
	}()
}

// startBlock spawns the per-block vuser pool, ramp governor, and completion timer.
func (r *Run) startBlock(block scenario.Block) {
	sem := newSemaphore(initialLimit(block))
	sel := r.newSelectorFor(block)

	r.mu.Lock()
	r.sems = append(r.sems, sem)
	if r.sem == nil {
		r.sem = sem
		r.selector = sel
	}
	r.mu.Unlock()
	for v := 0; v < block.Concurrency; v++ {
		vsess, sess, ok := r.buildSession(block, sel)
		if !ok {
			continue
		}
		// Pre-compute deps in the supervisor goroutine so all Rand.Split() calls
		// are sequential and never race with concurrent worker goroutines.
		deps := r.workerDeps(sem)
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			runWorker(r.runCtx, sess, vsess, deps)
		}()
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.runRampGovernor(block, sem)
	}()

	// Gate-aware completion: accumulates only active (un-paused) elapsed time so
	// operator-pause intervals do not count toward the block duration (design §6.4).
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.runBlockDuration(block)
	}()
}

// runBlockDuration accumulates active (un-paused) elapsed time toward the block's
// Duration. Each iteration waits for the gate to be open, starts a timer for the
// remaining duration, then selects on timer / gate-close / ctx. This implements
// the operator-pause timer shift (design §6.4): time spent paused is not counted.
// complete() is called in a separate goroutine to avoid deadlocking on r.wg.Wait
// (this goroutine is itself tracked by r.wg).
func (r *Run) runBlockDuration(block scenario.Block) {
	var elapsed time.Duration
	for elapsed < block.Duration {
		closedCh, err := r.gate.waitOpenAndGetCloseCh(r.runCtx)
		if err != nil {
			return
		}
		segStart := r.clk.Now()
		remaining := block.Duration - elapsed
		timer := r.clk.NewTimer(remaining)
		select {
		case <-r.runCtx.Done():
			timer.Stop()
			return
		case <-closedCh:
			timer.Stop()
			elapsed += r.clk.Since(segStart)
		case <-timer.C():
			elapsed = block.Duration
		}
	}
	if r.blocksLeft.Add(-1) == 0 {
		go r.complete()
	}
}

// complete drives the clean-completion path.
func (r *Run) complete() {
	if r.transitionTo(StateCompleting) != nil {
		return
	}
	r.cancel()
	r.wg.Wait()
	_ = r.transitionTo(StateCompleted)
	r.signalDone()
}

func (r *Run) signalDone() {
	r.doneOnce.Do(func() { close(r.done) })
}

// ID returns the run's crypto-random correlation id.
func (r *Run) ID() string { return r.id }

// AgentID returns the per-agent identity stamped on stat/audit records.
func (r *Run) AgentID() string { return r.agentID }

// Wait returns a channel closed when the run reaches a terminal state.
func (r *Run) Wait() <-chan struct{} { return r.done }

// Err returns the terminal error, or nil on clean stop/completion.
func (r *Run) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// Pause moves Running -> Paused and pauses the operator gate source.
func (r *Run) Pause() error {
	if err := r.transitionTo(StatePaused); err != nil {
		return err
	}
	r.gate.pause(gateOperator)
	return nil
}

// Resume moves Paused -> Running and re-opens the operator gate source.
func (r *Run) Resume() error {
	if err := r.transitionTo(StateRunning); err != nil {
		return err
	}
	r.gate.resume(gateOperator)
	return nil
}

// Stop drives the run to a terminal Stopped state: cancels runCtx then waits on
// the WaitGroup bounded by GraceTimeout. Stop is idempotent.
func (r *Run) Stop(ctx context.Context) error {
	if err := r.transitionTo(StateStopping); err != nil {
		// Already terminal (Stopped/Completed) or mid-completion: idempotent no-op.
		if s := r.State(); s == StateStopped || s == StateCompleted || s == StateCompleting {
			<-r.done
			return nil
		}
		return err
	}

	r.cancel()

	drained := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(drained)
	}()

	grace := r.clk.NewTimer(r.engine.opts.GraceTimeout)
	defer grace.Stop()

	select {
	case <-drained:
	case <-grace.C():
		r.setErr(pterr.New(pterr.ClassPermanent, "engine.grace", "engine.Stop",
			"grace deadline exceeded before all workers drained"))
		r.engine.opts.Logger.Error("engine stop: grace deadline exceeded", "run_id", r.id)
	case <-ctx.Done():
		r.setErr(pterr.Wrap(pterr.ClassPermanent, "engine.grace", "engine.Stop",
			"stop ctx canceled before drain", ctx.Err()))
	}

	_ = r.transitionTo(StateStopped)
	r.signalDone()
	return nil
}

func (r *Run) setErr(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err == nil {
		r.err = err
	}
}

// ----- supervisor helpers -----

func initialLimit(block scenario.Block) int {
	n := concurrencyAt(block.Ramp, block.Concurrency, 0)
	if n < 1 {
		return 1
	}
	return n
}

func (r *Run) newSelectorFor(block scenario.Block) *rotatingSelector {
	byProto := map[protocols.ProtocolID][]protocols.Target{block.Protocol: block.Targets}
	return newRotatingSelector(r.clk, r.engine.opts.Rand.Split(), block.Rotation, block.RotationInterval, byProto)
}

// Snapshot returns a point-in-time stats snapshot from the collector.
func (r *Run) Snapshot() StatsSnapshot { return r.coll.snapshot() }

// buildSession assembles a behavior.Session for one vuser.
func (r *Run) buildSession(block scenario.Block, sel behavior.TargetSelector) (behavior.Session, *protocols.Session, bool) {
	sid, err := idgen.SessionID()
	if err != nil {
		return nil, nil, false
	}
	deps := protocols.SessionDeps{
		Clock: r.clk,
		Rand:  r.engine.opts.Rand.Split(),
		Log:   r.engine.opts.Logger,
		Stats: r.coll,
		Audit: r.audit,
	}
	sess := &protocols.Session{
		ID:      protocols.SessionID(sid),
		Persona: block.Persona.Name,
		Targets: r.sc.Targets,
		States:  map[protocols.ProtocolID]protocols.SessionState{},
		Deps:    deps,
	}
	spec := block.Persona.ToSpec(sel)
	vsess, err := r.engine.opts.SessionMaker.NewSession(r.runCtx, spec, deps)
	if err != nil {
		return nil, nil, false
	}
	return vsess, sess, true
}

func (r *Run) workerDeps(sem *semaphore) workerDeps {
	return workerDeps{
		Clock:             r.clk,
		Rand:              r.engine.opts.Rand.Split(),
		Log:               r.engine.opts.Logger,
		Registry:          r.engine.opts.Registry,
		Limiter:           r.limiter,
		Sem:               sem,
		Stats:             r.coll,
		Gate:              r.gate,
		Breakers:          r.breakers,
		Tripwire:          r.tripwire,
		MaxRetries:        r.engine.opts.MaxRetries,
		BackoffBase:       r.engine.opts.BackoffBase,
		BackoffMax:        r.engine.opts.BackoffMax,
		PanicStorm:        8,
		SessionMaxActions: r.sc.Ceiling.PerSessionMaxActions,
	}
}

// runRampGovernor ticks on the clock and resizes the semaphore to concurrencyAt.
func (r *Run) runRampGovernor(block scenario.Block, sem *semaphore) {
	if block.Ramp.Up <= 0 {
		return // no ramp: initial limit is already the target
	}
	start := r.clk.Now()
	timer := r.clk.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-r.runCtx.Done():
			return
		case <-timer.C():
			elapsed := r.clk.Since(start)
			n := concurrencyAt(block.Ramp, block.Concurrency, elapsed)
			sem.setLimit(n)
			if elapsed >= block.Ramp.Up {
				return
			}
			timer = r.clk.NewTimer(time.Second)
		}
	}
}

// runScheduler pauses/resumes the schedule gate source on each active<->inactive edge.
func (r *Run) runScheduler() {
	for {
		now := r.clk.Now()
		if schedule.Active(r.sc.Schedule, now) {
			r.gate.resume(gateSchedule)
		} else {
			r.gate.pause(gateSchedule)
		}

		next := schedule.NextTransition(r.sc.Schedule, now)
		if next.IsZero() {
			return // no further transitions
		}
		d := next.Sub(now)
		if d <= 0 {
			d = time.Nanosecond
		}
		if err := r.clk.Sleep(r.runCtx, d); err != nil {
			return // run ctx canceled
		}
	}
}

// runStatsCollector publishes a snapshot on each clock tick.
func (r *Run) runStatsCollector() {
	timer := r.clk.NewTimer(r.engine.opts.StatsInterval)
	defer timer.Stop()
	for {
		select {
		case <-r.runCtx.Done():
			r.pub.publish(r.coll.snapshot())
			return
		case <-timer.C():
			r.pub.publish(r.coll.snapshot())
			timer = r.clk.NewTimer(r.engine.opts.StatsInterval)
		}
	}
}

func targetIDsOf(sc scenario.Scenario) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, b := range sc.Blocks {
		for _, t := range b.Targets {
			if _, ok := seen[t.ID]; ok {
				continue
			}
			seen[t.ID] = struct{}{}
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// keep imports active for helpers used above.
var _ = clock.NewReal
