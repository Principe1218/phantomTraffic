package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/audit"
	"github.com/Principe1218/phantomTraffic/internal/clock"
)

// recordingSink is an in-memory audit.Sink reused by every lifecycle test in
// this package. Defined here (Task E5.1); other *_test.go files use it as-is.
type recordingSink struct {
	mu     sync.Mutex
	events []audit.Event
}

func (s *recordingSink) Append(e audit.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}
func (s *recordingSink) Verify() error { return nil }
func (s *recordingSink) Close() error  { return nil }

func (s *recordingSink) actions() []audit.Action {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]audit.Action, len(s.events))
	for i, e := range s.events {
		out[i] = e.Action
	}
	return out
}

func TestStateString(t *testing.T) {
	cases := []struct {
		s    State
		want string
	}{
		{StateIdle, "idle"},
		{StateRunning, "running"},
		{StatePaused, "paused"},
		{StateStopping, "stopping"},
		{StateStopped, "stopped"},
		{StateCompleting, "completing"},
		{StateCompleted, "completed"},
		{State(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("State(%d).String() = %q, want %q", int32(tc.s), got, tc.want)
		}
	}
}

func TestTransitionToLegalAndIllegal(t *testing.T) {
	legal := []struct {
		from, to State
	}{
		{StateIdle, StateRunning},
		{StateRunning, StatePaused},
		{StatePaused, StateRunning},
		{StateRunning, StateStopping},
		{StatePaused, StateStopping},
		{StateStopping, StateStopped},
		{StateRunning, StateCompleting},
		{StateCompleting, StateCompleted},
	}
	for _, tc := range legal {
		r := newTestRun(t)
		r.state.Store(int32(tc.from))
		if err := r.transitionTo(tc.to); err != nil {
			t.Errorf("transitionTo(%s->%s) returned error %v, want nil", tc.from, tc.to, err)
		}
		if got := r.State(); got != tc.to {
			t.Errorf("after transitionTo(%s->%s) State() = %s, want %s", tc.from, tc.to, got, tc.to)
		}
	}

	illegal := []struct {
		from, to State
	}{
		{StateIdle, StatePaused},
		{StateStopped, StateRunning},
		{StateCompleted, StateRunning},
		{StateRunning, StateCompleted}, // must pass through Completing
		{StateStopping, StateRunning},
	}
	for _, tc := range illegal {
		r := newTestRun(t)
		r.state.Store(int32(tc.from))
		if err := r.transitionTo(tc.to); err == nil {
			t.Errorf("transitionTo(%s->%s) returned nil, want an illegal-transition error", tc.from, tc.to)
		}
		if got := r.State(); got != tc.from {
			t.Errorf("illegal transitionTo(%s->%s) mutated State() to %s, want unchanged %s", tc.from, tc.to, got, tc.from)
		}
	}
}

func TestTransitionToAppendsAudit(t *testing.T) {
	sink := &recordingSink{}
	r := newTestRun(t)
	r.audit = sink

	r.state.Store(int32(StateRunning))
	if err := r.transitionTo(StateStopping); err != nil {
		t.Fatalf("transitionTo(Running->Stopping): %v", err)
	}
	if err := r.transitionTo(StateStopped); err != nil {
		t.Fatalf("transitionTo(Stopping->Stopped): %v", err)
	}

	got := sink.actions()
	if len(got) != 2 {
		t.Fatalf("appended %d audit events, want 2: %v", len(got), got)
	}
	if got[1] != audit.ActionScenarioStopped {
		t.Errorf("final transition audit action = %q, want %q", got[1], audit.ActionScenarioStopped)
	}
}

// newTestRun builds a minimal *Run with just the fields the state machine needs:
// an atomic state, a no-op audit sink, a clock, and a run ID.
func newTestRun(t *testing.T) *Run {
	t.Helper()
	r := &Run{
		id:    "run-test",
		clk:   clock.NewReal(),
		audit: &recordingSink{},
		done:  make(chan struct{}),
	}
	r.state.Store(int32(StateIdle))
	_ = time.Second // keep the time import live for later tasks editing this file
	return r
}
