package behavior

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// goldenSpec exercises every primitive: a 2-protocol weighted mix, LogNormal
// think-time (a Norm draw), proportional jitter (a Float draw), a renewal burst,
// and a multi-target round-robin selector. Each call returns FRESH stateful
// components so two runs are independent.
func goldenSpec(t *testing.T) SessionSpec {
	t.Helper()
	mix, err := NewTemplateMix([]Template{
		{Protocol: protocols.ProtoHTTP, Verb: "fetch-page", Cause: protocols.CauseNavigation, Pacing: protocols.PacingShaperManaged, Weight: 2},
		{Protocol: protocols.ProtoSSH, Verb: "run", Cause: protocols.CauseNavigation, Pacing: protocols.PacingShaperManaged, Weight: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	sel := NewRoundRobinSelector(map[protocols.ProtocolID][]protocols.Target{
		protocols.ProtoHTTP: {
			{ID: "web1", Proto: protocols.ProtoHTTP, Addr: "web1:443"},
			{ID: "web2", Proto: protocols.ProtoHTTP, Addr: "web2:443"},
		},
		protocols.ProtoSSH: {{ID: "jump", Proto: protocols.ProtoSSH, Addr: "jump:22"}},
	})
	return SessionSpec{
		Mix:       mix,
		ThinkTime: LogNormal{Mu: 0.5, Sigma: 0.4, Scale: time.Second},
		Jitter:    ProportionalJitter{Fraction: 0.1},
		Burst:     NewRenewalBurst(Constant{D: 5 * time.Second}, Constant{D: 30 * time.Second}),
		TimeOfDay: FlatTimeOfDay{},
		Bounds:    DefaultBranchBounds(),
		Selector:  sel,
	}
}

// runSession drives a fresh session n steps, fast-forwarding the fake clock by
// each Step.Wait exactly as the engine would by sleeping.
func runSession(t *testing.T, spec SessionSpec, script rng.FakeScript, n int) []Step {
	t.Helper()
	fc := clock.NewFake(sessBase)
	deps := protocols.SessionDeps{Clock: fc, Rand: rng.NewFake(script)}
	s, err := NewSessionMaker().NewSession(context.Background(), spec, deps)
	if err != nil {
		t.Fatal(err)
	}
	var steps []Step
	for i := 0; i < n; i++ {
		st, err := s.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, st)
		if st.Done {
			break
		}
		fc.Advance(st.Wait)
	}
	return steps
}

func TestSessionGoldenSequenceIsDeterministic(t *testing.T) {
	// Per navigation step the session draws: mix.Pick (Float) + think (Norm) +
	// jitter (Float) = 2 Floats + 1 Norm. 5 steps -> 10 Floats + 5 Norms; the
	// script provides a few extra.
	script := rng.FakeScript{
		Floats: []float64{0.10, 0.55, 0.80, 0.20, 0.95, 0.33, 0.67, 0.41, 0.05, 0.72, 0.88, 0.15},
		Norms:  []float64{0.2, -0.3, 0.7, -0.1, 0.4, 0.9},
	}
	const steps = 5

	a := runSession(t, goldenSpec(t), script, steps)
	b := runSession(t, goldenSpec(t), script, steps)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("identical seed produced divergent sequences:\n a=%+v\n b=%+v", a, b)
	}
	if len(a) != steps {
		t.Fatalf("expected %d steps, got %d", steps, len(a))
	}

	// Every emitted action targets an allowlisted host (by construction).
	allowed := map[string]bool{"web1": true, "web2": true, "jump": true}
	emitted := 0
	for _, st := range a {
		if st.Action == nil {
			continue
		}
		emitted++
		if !allowed[st.Action.Target.ID] {
			t.Fatalf("off-allowlist target %q", st.Action.Target.ID)
		}
		if st.Action.Params != nil {
			t.Fatal("Params must be nil in Plan 3")
		}
	}
	if emitted == 0 {
		t.Fatal("expected at least one action step in the golden sequence")
	}
}
