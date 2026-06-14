package behavior

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

var sessBase = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// oneHTTPSelector builds a selector with a single http target.
func oneHTTPSelector() *RoundRobinSelector {
	return NewRoundRobinSelector(map[protocols.ProtocolID][]protocols.Target{
		protocols.ProtoHTTP: {{ID: "t", Proto: protocols.ProtoHTTP, Addr: "t:443"}},
	})
}

// httpMix is a single-template navigation mix.
func httpMix(t *testing.T) TemplateMix {
	m, err := NewTemplateMix([]Template{
		{Protocol: protocols.ProtoHTTP, Verb: "fetch-page", Cause: protocols.CauseNavigation, Weight: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func newTestSession(t *testing.T, spec SessionSpec, script rng.FakeScript) (Session, *clock.FakeClock) {
	t.Helper()
	fc := clock.NewFake(sessBase)
	deps := protocols.SessionDeps{Clock: fc, Rand: rng.NewFake(script)}
	s, err := NewSessionMaker().NewSession(context.Background(), spec, deps)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return s, fc
}

func TestSessionEmitsActionStep(t *testing.T) {
	spec := SessionSpec{
		Mix: httpMix(t), ThinkTime: Constant{D: time.Second},
		Jitter: NoJitter{}, Burst: AlwaysActive{}, TimeOfDay: FlatTimeOfDay{},
		Bounds: DefaultBranchBounds(), Selector: oneHTTPSelector(),
	}
	s, _ := newTestSession(t, spec, rng.FakeScript{Floats: []float64{0.5}}) // mix.Pick
	step, err := s.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if step.Done || step.Action == nil {
		t.Fatalf("expected an action step, got %+v", step)
	}
	if step.Wait != time.Second {
		t.Fatalf("wait = %v, want 1s", step.Wait)
	}
	if step.Action.Ref.String() != "http:fetch-page" || step.Action.Target.ID != "t" {
		t.Fatalf("action = %+v", step.Action)
	}
	if step.Action.Params != nil {
		t.Fatal("Params must be nil in Plan 3")
	}
}

func TestSessionIdleStepFromBurst(t *testing.T) {
	burst := &scriptBurst{phases: []BurstPhase{{Idle: true, IdleFor: 500 * time.Millisecond}}}
	spec := SessionSpec{
		Mix: httpMix(t), ThinkTime: Constant{D: time.Second},
		Jitter: NoJitter{}, Burst: burst, TimeOfDay: FlatTimeOfDay{},
		Bounds: DefaultBranchBounds(), Selector: oneHTTPSelector(),
	}
	s, _ := newTestSession(t, spec, rng.FakeScript{Floats: []float64{0.5}})
	step, _ := s.Next(context.Background())
	if step.Action != nil || step.Wait != 500*time.Millisecond {
		t.Fatalf("expected idle step of 500ms, got %+v", step)
	}
}

func TestSessionSkipsProtocolWithNoTargets(t *testing.T) {
	spec := SessionSpec{
		Mix: httpMix(t), ThinkTime: Constant{D: time.Second},
		Jitter: NoJitter{}, Burst: AlwaysActive{}, TimeOfDay: FlatTimeOfDay{},
		Bounds:   DefaultBranchBounds(),
		Selector: NewRoundRobinSelector(map[protocols.ProtocolID][]protocols.Target{}), // empty
	}
	s, _ := newTestSession(t, spec, rng.FakeScript{Floats: []float64{0.5}})
	step, _ := s.Next(context.Background())
	if step.Action != nil || step.Done {
		t.Fatalf("expected a benign skip (no action), got %+v", step)
	}
}

func TestSessionTerminatesByLength(t *testing.T) {
	spec := SessionSpec{
		Mix: httpMix(t), ThinkTime: Constant{D: time.Second},
		Jitter: NoJitter{}, Burst: AlwaysActive{}, TimeOfDay: FlatTimeOfDay{},
		Shape:  SessionShape{Length: Constant{D: 10 * time.Second}},
		Bounds: DefaultBranchBounds(), Selector: oneHTTPSelector(),
	}
	s, fc := newTestSession(t, spec, rng.FakeScript{Floats: []float64{0.5, 0.5}})
	if step, _ := s.Next(context.Background()); step.Done {
		t.Fatal("must not be done at t=0")
	}
	fc.Advance(10 * time.Second)
	if step, _ := s.Next(context.Background()); !step.Done {
		t.Fatal("must be done once length elapses")
	}
}

func TestSessionTerminatesByAbandon(t *testing.T) {
	spec := SessionSpec{
		Mix: httpMix(t), ThinkTime: Constant{D: time.Second},
		Jitter: NoJitter{}, Burst: AlwaysActive{}, TimeOfDay: FlatTimeOfDay{},
		Shape:  SessionShape{Abandon: 1.0}, // always abandons
		Bounds: DefaultBranchBounds(), Selector: oneHTTPSelector(),
	}
	s, _ := newTestSession(t, spec, rng.FakeScript{Floats: []float64{0.0}}) // abandon draw
	if step, _ := s.Next(context.Background()); !step.Done {
		t.Fatal("Abandon=1.0 must terminate on the first step")
	}
}

func TestSessionObserveDrivesPriorReactiveThink(t *testing.T) {
	spec := SessionSpec{
		Mix: httpMix(t), ThinkTime: Constant{D: time.Second},
		Jitter: NoJitter{}, Burst: AlwaysActive{}, TimeOfDay: FlatTimeOfDay{},
		Bounds: DefaultBranchBounds(), Selector: oneHTTPSelector(),
	}
	s, _ := newTestSession(t, spec, rng.FakeScript{Floats: []float64{0.5}})
	s.Observe(protocols.Result{Outcome: protocols.OutcomeFailure}, protocols.Observation{})
	step, _ := s.Next(context.Background())
	if step.Action == nil || step.Wait != 2*time.Second {
		t.Fatalf("a failed prior action must double the next think to 2s, got %+v", step)
	}
}

func TestNewSessionRequiresSelectorAndDeps(t *testing.T) {
	good := protocols.SessionDeps{Clock: clock.NewFake(sessBase), Rand: rng.NewFake(rng.FakeScript{})}
	if _, err := NewSessionMaker().NewSession(context.Background(), SessionSpec{Mix: httpMix(t)}, good); err == nil {
		t.Fatal("expected error when Selector is nil")
	}
	spec := SessionSpec{Mix: httpMix(t), Selector: oneHTTPSelector()}
	if _, err := NewSessionMaker().NewSession(context.Background(), spec, protocols.SessionDeps{}); err == nil {
		t.Fatal("expected error when Clock/Rand are nil")
	}
}

func TestNewSessionRequiresNonEmptyMix(t *testing.T) {
	spec := SessionSpec{
		// zero-value TemplateMix — Len() == 0
		Selector: oneHTTPSelector(),
	}
	deps := protocols.SessionDeps{Clock: clock.NewFake(sessBase), Rand: rng.NewFake(rng.FakeScript{})}
	_, err := NewSessionMaker().NewSession(context.Background(), spec, deps)
	if err == nil {
		t.Fatal("expected error when Mix is empty")
	}
}

func TestSessionFingerprintAndBounds(t *testing.T) {
	pool, err := DefaultFingerprintPool()
	if err != nil {
		t.Fatal(err)
	}
	spec := SessionSpec{
		Mix: httpMix(t), ThinkTime: Constant{D: time.Second},
		Jitter: NoJitter{}, Burst: AlwaysActive{}, TimeOfDay: FlatTimeOfDay{},
		Prints: pool, Bounds: DefaultBranchBounds(), Selector: oneHTTPSelector(),
	}
	s, _ := newTestSession(t, spec, rng.FakeScript{Ints: []int{0}})
	fp := s.Fingerprint()
	if fp.UserAgent == "" {
		t.Fatal("expected a non-empty fingerprint")
	}
	if s.Fingerprint() != fp {
		t.Fatal("fingerprint must be stable for the whole session")
	}
	if s.Bounds() != DefaultBranchBounds() {
		t.Fatalf("Bounds() = %+v", s.Bounds())
	}
}
