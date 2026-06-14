package behavior

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// constTOD is a fixed-intensity TimeOfDayShaper test double.
type constTOD float64

func (c constTOD) Intensity(time.Time) float64 { return float64(c) }
func (c constTOD) Name() string                { return "const" }

// scriptBurst replays scripted BurstPhases, then stays active.
type scriptBurst struct {
	phases []BurstPhase
	i      int
}

func (s *scriptBurst) Phase(time.Time, rng.Rand) BurstPhase {
	if s.i >= len(s.phases) {
		return BurstPhase{}
	}
	p := s.phases[s.i]
	s.i++
	return p
}
func (s *scriptBurst) Name() string { return "script" }

func navCtx(base time.Duration, prior *protocols.Result, r rng.Rand) ShapeCtx {
	return ShapeCtx{Now: baseTime, Think: func() time.Duration { return base }, Cause: protocols.CauseNavigation, Prior: prior, Rand: r}
}

var baseTime = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

func TestChainShaperCauseBranches(t *testing.T) {
	r := rng.NewFake(rng.FakeScript{Floats: []float64{0.5}})
	s := NewChainShaper(NoJitter{}, AlwaysActive{}, FlatTimeOfDay{})

	// Control: no human gap.
	if d := s.Shape(ShapeCtx{Cause: protocols.CauseControl, Rand: r}); d.Wait != 0 || d.Idle {
		t.Fatalf("control: %+v", d)
	}
	// Sub-resource: micro-jitter only (0.5 * 15ms = 7.5ms).
	if d := s.Shape(ShapeCtx{Cause: protocols.CauseSubResource, Rand: r}); d.Wait != 7500*time.Microsecond || d.Idle {
		t.Fatalf("sub-resource: %+v", d)
	}
}

func TestChainShaperNavigationPipeline(t *testing.T) {
	r := rng.NewFake(rng.FakeScript{})

	// Baseline: NoJitter, no burst, peak intensity, no prior -> BaseThink verbatim.
	s := NewChainShaper(NoJitter{}, AlwaysActive{}, FlatTimeOfDay{})
	if d := s.Shape(navCtx(time.Second, nil, r)); d.Wait != time.Second {
		t.Fatalf("baseline wait = %v, want 1s", d.Wait)
	}

	// Quiet hour (intensity 0.5) stretches the wait: 1s / 0.5 = 2s.
	s = NewChainShaper(NoJitter{}, AlwaysActive{}, constTOD(0.5))
	if d := s.Shape(navCtx(time.Second, nil, r)); d.Wait != 2*time.Second {
		t.Fatalf("quiet-hour wait = %v, want 2s", d.Wait)
	}

	// A failed prior action lengthens the next pause (x2).
	s = NewChainShaper(NoJitter{}, AlwaysActive{}, FlatTimeOfDay{})
	prior := &protocols.Result{Outcome: protocols.OutcomeFailure}
	if d := s.Shape(navCtx(time.Second, prior, r)); d.Wait != 2*time.Second {
		t.Fatalf("post-failure wait = %v, want 2s", d.Wait)
	}

	// A burst trough short-circuits to an idle Step.
	burst := &scriptBurst{phases: []BurstPhase{{Idle: true, IdleFor: 500 * time.Millisecond}}}
	s = NewChainShaper(NoJitter{}, burst, FlatTimeOfDay{})
	if d := s.Shape(navCtx(time.Second, nil, r)); !d.Idle || d.Wait != 500*time.Millisecond {
		t.Fatalf("burst idle: %+v", d)
	}
}
