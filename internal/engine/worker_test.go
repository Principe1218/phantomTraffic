package engine

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
	"github.com/Principe1218/phantomTraffic/internal/safety"
)

// ----- shared in-test fakes (reused by E4.4-E4.7) -----

const testProto protocols.ProtocolID = "noop"

// fakeVSession is a scripted behavior.Session. Each call to Next returns the next
// scripted Step; when the script is exhausted it returns a Done step. It records
// every Result handed to Observe so a test can assert the closed loop fired.
type fakeVSession struct {
	steps    []behavior.Step
	idx      int
	observed []protocols.Result
}

func (f *fakeVSession) Next(ctx context.Context) (behavior.Step, error) {
	if err := ctx.Err(); err != nil {
		return behavior.Step{}, err
	}
	if f.idx >= len(f.steps) {
		return behavior.Step{Done: true}, nil
	}
	s := f.steps[f.idx]
	f.idx++
	return s, nil
}

func (f *fakeVSession) Observe(res protocols.Result, _ protocols.Observation) {
	f.observed = append(f.observed, res)
}

func (f *fakeVSession) Fingerprint() behavior.Fingerprint { return behavior.Fingerprint{} }
func (f *fakeVSession) Bounds() behavior.BranchBounds     { return behavior.BranchBounds{} }

// fakeAction is a minimal concrete protocols.Action carried by a scripted Step.
type fakeAction struct {
	protocols.BaseAction
}

func (fakeAction) Kind() protocols.ActionKind { return "ping" }
func (fakeAction) Validate() error            { return nil }

func newStepWithAction(target protocols.Target) behavior.Step {
	return behavior.Step{
		Action: &behavior.PlannedAction{
			Ref:    protocols.Ref{Protocol: testProto, Verb: "ping"},
			Params: fakeAction{BaseAction: protocols.BaseAction{Proto: testProto}},
			Target: target,
			Label:  "noop:ping",
		},
	}
}

// fakeHandler is a scripted protocols.ProtocolHandler. doFn computes the
// (Result, Observation, error) for each Do call; it sees the 0-based call index so
// a test can vary behavior per attempt (transient-then-success in E4.5). doFn may
// be nil for the always-succeed default. If panicOnCall >= 0, the matching Do call
// panics (E4.3 path through the worker is exercised indirectly).
type fakeHandler struct {
	mu          sync.Mutex
	calls       int
	doFn        func(call int) (protocols.Result, protocols.Observation, error)
	panicOnCall int
}

func (*fakeHandler) ID() protocols.ProtocolID { return testProto }
func (*fakeHandler) Capability() protocols.Capability {
	return protocols.Capability{Proto: testProto, Actions: []protocols.ActionKind{"ping"}}
}

func (*fakeHandler) OpenState(ctx context.Context, _ *protocols.Session) (protocols.SessionState, error) {
	// SessionState is sealed to package protocols; runAction never calls OpenState,
	// so nil is safe for worker tests.
	return nil, ctx.Err()
}
func (*fakeHandler) CloseState(context.Context, protocols.SessionState) error { return nil }

func (h *fakeHandler) Do(ctx context.Context, _ *protocols.Session, _ protocols.Action) (protocols.Result, protocols.Observation, error) {
	h.mu.Lock()
	call := h.calls
	h.calls++
	h.mu.Unlock()
	if h.panicOnCall == call {
		panic("fakeHandler scripted panic")
	}
	if h.doFn != nil {
		return h.doFn(call)
	}
	return protocols.Result{Protocol: testProto, Action: "ping", Outcome: protocols.OutcomeSuccess}, protocols.Observation{}, nil
}

func (h *fakeHandler) callCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.calls
}

// testTarget is the single target used across the worker tests.
func testTarget() protocols.Target {
	return protocols.Target{ID: "t1", Proto: testProto, Addr: "noop.test"}
}

// newTestSession builds a minimal *protocols.Session for the worker under test.
func newTestSession(clk clock.Clock, r rng.Rand) *protocols.Session {
	return &protocols.Session{
		ID:      "sess-1",
		Persona: "test",
		States:  map[protocols.ProtocolID]protocols.SessionState{},
		Deps: protocols.SessionDeps{
			Clock: clk,
			Rand:  r,
			Log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
	}
}

// newWorkerDeps assembles workerDeps with real (non-faked) safety primitives over
// the fake clock, a single-target collector, an open gate, and the given handler.
func newWorkerDeps(t *testing.T, clk *clock.FakeClock, r rng.Rand, h protocols.ProtocolHandler, breaker *safety.Breaker, tw *safety.Tripwire) workerDeps {
	t.Helper()
	reg := protocols.NewRegistry()
	if err := reg.Register(h); err != nil {
		t.Fatalf("register handler: %v", err)
	}
	if tw == nil {
		tw = safety.NewTripwire(0) // unlimited budget
	}
	if breaker == nil {
		breaker = safety.NewBreaker(clk, 2, time.Minute)
	}
	caps := safety.CapSpec{} // no rate limit configured => limiter never blocks/trips on caps
	return workerDeps{
		Clock:             clk,
		Rand:              r,
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		Registry:          reg,
		Limiter:           safety.NewLimiter(clk, caps, tw),
		Sem:               newSemaphore(16),
		Stats:             newCollector([]string{"t1"}, clk, func() float64 { return 0 }),
		Gate:              newGate(clk),
		Breakers:          map[string]*safety.Breaker{"t1": breaker},
		Tripwire:          tw,
		MaxRetries:        3,
		BackoffBase:       10 * time.Millisecond,
		BackoffMax:        time.Second,
		PanicStorm:        4,
		SessionMaxActions: 100,
	}
}

// runWorkerAsync runs the worker in a goroutine and drives the fake clock forward
// so any clock.Sleep(Wait) calls unblock. It returns a channel closed when the
// worker returns. Steps here use Wait==0 so no clock advance is strictly required,
// but we advance once to be safe with the register-before-advance idiom.
func runWorkerAsync(ctx context.Context, sess *protocols.Session, vs behavior.Session, d workerDeps, clk *clock.FakeClock) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		runWorker(ctx, sess, vs, d)
	}()
	return done
}

// ----- E4.4: happy path -----

func TestRunWorkerHappyPath(t *testing.T) {
	clk := clock.NewFake(time.Unix(0, 0).UTC())
	r := rng.NewFake(rng.FakeScript{}) // no draws expected on the happy path
	h := &fakeHandler{panicOnCall: -1}

	d := newWorkerDeps(t, clk, r, h, nil, nil)
	tgt := testTarget()
	vs := &fakeVSession{steps: []behavior.Step{
		newStepWithAction(tgt),
		newStepWithAction(tgt),
		newStepWithAction(tgt),
		{Done: true},
	}}
	sess := newTestSession(clk, r)

	done := runWorkerAsync(context.Background(), sess, vs, d, clk)
	// No Wait on these steps, but advance once to honor the idiom for any sleeps.
	time.Sleep(20 * time.Millisecond)
	clk.Advance(time.Second)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runWorker did not return after the Done step")
	}

	if h.callCount() != 3 {
		t.Fatalf("handler Do called %d times, want 3", h.callCount())
	}
	if len(vs.observed) != 3 {
		t.Fatalf("session observed %d results, want 3", len(vs.observed))
	}

	snap := d.Stats.snapshot()
	if snap.Successes != 3 {
		t.Fatalf("snapshot Successes = %d, want 3", snap.Successes)
	}
	if snap.Failures != 0 {
		t.Fatalf("snapshot Failures = %d, want 0", snap.Failures)
	}
	for _, res := range vs.observed {
		if res.Outcome != protocols.OutcomeSuccess {
			t.Fatalf("observed outcome %v, want OutcomeSuccess", res.Outcome)
		}
	}
}
