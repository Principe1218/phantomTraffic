package behavior

import (
	"time"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// BurstModel gates whether the session is in an idle trough, so traffic clusters
// then goes quiet rather than streaming at a metronomic cadence. Phase is called
// once per navigation step with a monotonically non-decreasing logical `now`.
type BurstModel interface {
	Phase(now time.Time, r rng.Rand) BurstPhase
	Name() string
	// Clone returns a fresh copy of this model with the same configuration but
	// independent mutable state. The engine calls Clone once per vuser session so
	// concurrent Phase calls from different goroutines never race.
	Clone() BurstModel
}

// BurstPhase is the renewal verdict for one step. Idle is true only at the
// transition INTO a trough; IdleFor is the trough duration the Session emits as a
// single idle Step (after which logical time advances past the trough).
type BurstPhase struct {
	Idle    bool
	IdleFor time.Duration
}

// RenewalBurst is an on/off renewal process: an Active-dwell window of normal
// activity followed by an Idle-dwell trough, repeating. Both dwells are
// Distributions sampled from the injected rng.Rand, so the process is fully
// deterministic. State is internal and advanced only by Phase.
type RenewalBurst struct {
	Active Distribution
	Idle   Distribution

	started  bool
	idle     bool
	phaseEnd time.Time
}

// NewRenewalBurst builds a renewal burst with the given active/idle dwell
// distributions.
func NewRenewalBurst(active, idle Distribution) *RenewalBurst {
	return &RenewalBurst{Active: active, Idle: idle}
}

func (b *RenewalBurst) Phase(now time.Time, r rng.Rand) BurstPhase {
	if !b.started {
		b.started = true
		b.idle = false
		b.phaseEnd = now.Add(b.Active.Sample(r))
		return BurstPhase{}
	}
	if now.Before(b.phaseEnd) {
		return BurstPhase{} // still within the current phase
	}
	if b.idle {
		// idle trough elapsed -> begin a new active window
		b.idle = false
		b.phaseEnd = now.Add(b.Active.Sample(r))
		return BurstPhase{}
	}
	// active window elapsed -> enter an idle trough
	b.idle = true
	d := b.Idle.Sample(r)
	b.phaseEnd = now.Add(d)
	return BurstPhase{Idle: true, IdleFor: d}
}

func (b *RenewalBurst) Name() string { return "renewal" }

// Clone returns a new *RenewalBurst with the same Active/Idle distributions but
// fresh (unstarted) state so each vuser session owns independent mutable fields.
func (b *RenewalBurst) Clone() BurstModel {
	return &RenewalBurst{Active: b.Active, Idle: b.Idle}
}

// AlwaysActive never injects an idle trough (for personas without burstiness).
type AlwaysActive struct{}

func (AlwaysActive) Phase(time.Time, rng.Rand) BurstPhase { return BurstPhase{} }
func (AlwaysActive) Name() string                         { return "always-active" }
func (AlwaysActive) Clone() BurstModel                    { return AlwaysActive{} }
