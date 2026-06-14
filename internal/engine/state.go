package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/safety"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// State is the run lifecycle state. It is stored as an atomic.Int32 on the Run so
// State() is a lock-free read (design §9); legal transitions are mutex-guarded.
type State int32

const (
	StateIdle State = iota
	StateRunning
	StatePaused
	StateStopping
	StateStopped
	StateCompleting
	StateCompleted
)

// String returns the lowercase audit-friendly name of the state.
func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateRunning:
		return "running"
	case StatePaused:
		return "paused"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateCompleting:
		return "completing"
	case StateCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

// legalTransitions is the closed legal-transition table. Default is DENY (a
// transition absent from this map is illegal) — AGENTS.md §4.2.
var legalTransitions = map[State]map[State]struct{}{
	StateIdle:       {StateRunning: {}},
	StateRunning:    {StatePaused: {}, StateStopping: {}, StateCompleting: {}},
	StatePaused:     {StateRunning: {}, StateStopping: {}},
	StateStopping:   {StateStopped: {}},
	StateCompleting: {StateCompleted: {}},
	StateStopped:    {},
	StateCompleted:  {},
}

// auditActionFor maps a destination state to the audit Action emitted on entry.
func auditActionFor(to State) (audit.Action, bool) {
	switch to {
	case StateRunning:
		return audit.ActionScenarioStarted, true
	case StateStopping:
		return audit.ActionScenarioStopped, true
	case StateStopped:
		return audit.ActionScenarioStopped, true
	case StateCompleted:
		return audit.ActionScenarioStopped, true
	default:
		return "", false
	}
}

// transitionTo applies a state change if it is legal, then appends an audit event
// on success. It is mutex-guarded so concurrent lifecycle calls serialize.
func (r *Run) transitionTo(to State) error {
	r.transMu.Lock()
	defer r.transMu.Unlock()

	from := State(r.state.Load())
	allowed, ok := legalTransitions[from]
	if !ok {
		return fmt.Errorf("engine: no transitions defined from state %s", from)
	}
	if _, ok := allowed[to]; !ok {
		return fmt.Errorf("engine: illegal transition %s -> %s", from, to)
	}
	r.state.Store(int32(to))

	if action, audited := auditActionFor(to); audited && r.audit != nil {
		_ = r.audit.Append(audit.Event{
			Actor:    "engine",
			Action:   action,
			Resource: r.id,
			Detail:   map[string]string{"from": from.String(), "to": to.String()},
		})
	}
	return nil
}

// Run is the engine's per-run handle. All fields are populated by Engine.Start.
type Run struct {
	id      string
	agentID string
	clk     clock.Clock
	audit   audit.Sink
	engine  *Engine
	sc      scenario.Scenario

	runCtx context.Context    //nolint:containedctx
	cancel context.CancelFunc

	gate     *gate
	coll     *collector
	pub      *publisher
	limiter  safety.Limiter
	breakers map[string]*safety.Breaker
	tripwire *safety.Tripwire

	mu   sync.Mutex // guards err, sems
	err  error
	sems []*semaphore

	// patchMu is the single write barrier for all ApplyPatch mutations. The hot
	// path (Record, gate) never holds it; it is only held during a live patch.
	patchMu     sync.Mutex
	caps        safety.CapSpec
	capOverride bool
	sem         *semaphore        // first block's semaphore; set by startBlock
	weights     map[string]uint   // block ID -> weight; set by startSupervisor
	selector    *rotatingSelector // first block's selector; set by startBlock

	wg         sync.WaitGroup
	blocksLeft atomic.Int64

	state    atomic.Int32
	transMu  sync.Mutex
	done     chan struct{}
	doneOnce sync.Once
}

// State returns the current lifecycle state with a lock-free atomic read.
func (r *Run) State() State { return State(r.state.Load()) }
