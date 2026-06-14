package behavior

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// base is a fixed logical time; the renewal model is a pure function of the
// monotonic `now` argument, so tests advance it by hand.
var base = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC) // a Monday

func TestRenewalBurstClustersThenIdles(t *testing.T) {
	r := rng.NewFake(rng.FakeScript{})
	b := NewRenewalBurst(Constant{D: 100 * time.Millisecond}, Constant{D: 500 * time.Millisecond})

	// First call begins an active window; never idle.
	if ph := b.Phase(base, r); ph.Idle {
		t.Fatalf("first Phase must not be idle: %+v", ph)
	}
	// Still inside the active window.
	if ph := b.Phase(base.Add(50*time.Millisecond), r); ph.Idle {
		t.Fatalf("mid-active Phase must not be idle: %+v", ph)
	}
	// Active window elapsed -> enter an idle trough of the idle dwell.
	ph := b.Phase(base.Add(100*time.Millisecond), r)
	if !ph.Idle || ph.IdleFor != 500*time.Millisecond {
		t.Fatalf("expected idle trough 500ms, got %+v", ph)
	}
	// After the trough -> back to active, not idle.
	if ph := b.Phase(base.Add(600*time.Millisecond), r); ph.Idle {
		t.Fatalf("post-trough Phase must be active: %+v", ph)
	}
}

func TestAlwaysActiveNeverIdles(t *testing.T) {
	r := rng.NewFake(rng.FakeScript{})
	if ph := (AlwaysActive{}).Phase(base, r); ph.Idle {
		t.Fatalf("AlwaysActive must never idle: %+v", ph)
	}
	if (AlwaysActive{}).Name() != "always-active" {
		t.Fatalf("name = %q", (AlwaysActive{}).Name())
	}
}

func TestRenewalBurstCloneIsIndependent(t *testing.T) {
	r := rng.NewFake(rng.FakeScript{})
	b := NewRenewalBurst(Constant{D: 100 * time.Millisecond}, Constant{D: 500 * time.Millisecond})
	c := b.Clone()

	// Drive original past its active window into an idle trough.
	b.Phase(base, r)
	b.Phase(base.Add(100*time.Millisecond), r)

	// Clone must start fresh: first Phase call is always active, never idle.
	if ph := c.Phase(base, r); ph.Idle {
		t.Fatal("Clone shares state with original: got idle on first call, want active")
	}
	if c.Name() != "renewal" {
		t.Fatalf("Clone().Name() = %q, want renewal", c.Name())
	}
}

func TestAlwaysActiveClone(t *testing.T) {
	c := (AlwaysActive{}).Clone()
	if c.Name() != "always-active" {
		t.Fatalf("Clone().Name() = %q, want always-active", c.Name())
	}
	// Must still be stateless: never idles.
	r := rng.NewFake(rng.FakeScript{})
	if ph := c.Phase(base, r); ph.Idle {
		t.Fatal("cloned AlwaysActive must never idle")
	}
}
