package engine

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// httpTargets builds a small, ordered target list for the http protocol.
func httpTargets(ids ...string) []protocols.Target {
	out := make([]protocols.Target, 0, len(ids))
	for _, id := range ids {
		out = append(out, protocols.Target{ID: id, Proto: protocols.ProtoHTTP, Addr: id + ":80"})
	}
	return out
}

// TestRotatingSelectorSequentialStaysThenAdvances verifies that the sequential
// selector returns a stable target until the rotation interval has elapsed on the
// injected clock, then advances to the next index, wrapping modulo n.
func TestRotatingSelectorSequentialStaysThenAdvances(t *testing.T) {
	start := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(start)
	r := rng.NewFake(rng.FakeScript{}) // sequential draws no randomness
	interval := 5 * time.Minute
	byProto := map[protocols.ProtocolID][]protocols.Target{
		protocols.ProtoHTTP: httpTargets("t0", "t1", "t2"),
	}

	s := newRotatingSelector(clk, r, scenario.RotationSequential, interval, byProto)

	// First pick is index 0 and stays there across repeated calls before any time passes.
	for i := 0; i < 3; i++ {
		got, ok := s.Next(protocols.ProtoHTTP)
		if !ok {
			t.Fatalf("call %d: ok=false, want a target", i)
		}
		if got.ID != "t0" {
			t.Fatalf("call %d: got %q, want %q (no time elapsed)", i, got.ID, "t0")
		}
	}

	// Advancing less than the interval must NOT rotate.
	clk.Advance(interval - time.Nanosecond)
	if got, _ := s.Next(protocols.ProtoHTTP); got.ID != "t0" {
		t.Fatalf("partial-interval: got %q, want %q", got.ID, "t0")
	}

	// Crossing the interval rotates to index 1.
	clk.Advance(time.Nanosecond)
	if got, _ := s.Next(protocols.ProtoHTTP); got.ID != "t1" {
		t.Fatalf("first rotation: got %q, want %q", got.ID, "t1")
	}

	// Another full interval rotates to index 2.
	clk.Advance(interval)
	if got, _ := s.Next(protocols.ProtoHTTP); got.ID != "t2" {
		t.Fatalf("second rotation: got %q, want %q", got.ID, "t2")
	}

	// A third full interval wraps back to index 0.
	clk.Advance(interval)
	if got, _ := s.Next(protocols.ProtoHTTP); got.ID != "t0" {
		t.Fatalf("wrap rotation: got %q, want %q", got.ID, "t0")
	}
}

// TestRotatingSelectorImplementsTargetSelector pins the selector to the Plan-3
// behavior.TargetSelector contract at compile time.
func TestRotatingSelectorImplementsTargetSelector(t *testing.T) {
	var _ behavior.TargetSelector = (*rotatingSelector)(nil)
}

// TestRotatingSelectorRandomPicksOnInterval verifies that the random strategy draws
// a fresh index from the injected RNG only when the interval elapses, and stays put
// otherwise. The FakeScript supplies in-range indices [0,n); the n arg is ignored by
// the fake, so scripted values must already be in range.
func TestRotatingSelectorRandomPicksOnInterval(t *testing.T) {
	start := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(start)
	// Three targets => indices 0..2. Script the two rotations we will trigger.
	r := rng.NewFake(rng.FakeScript{Ints: []int{2, 0}})
	interval := 1 * time.Minute
	byProto := map[protocols.ProtocolID][]protocols.Target{
		protocols.ProtoHTTP: httpTargets("a", "b", "c"),
	}

	s := newRotatingSelector(clk, r, scenario.RotationRandom, interval, byProto)

	// Before any time elapses the selector stays on index 0 and draws nothing.
	if got, _ := s.Next(protocols.ProtoHTTP); got.ID != "a" {
		t.Fatalf("initial: got %q, want %q", got.ID, "a")
	}

	// First interval => first scripted draw (2 => "c").
	clk.Advance(interval)
	if got, _ := s.Next(protocols.ProtoHTTP); got.ID != "c" {
		t.Fatalf("first random pick: got %q, want %q", got.ID, "c")
	}
	// No further time => no draw; stays on "c".
	if got, _ := s.Next(protocols.ProtoHTTP); got.ID != "c" {
		t.Fatalf("hold after pick: got %q, want %q", got.ID, "c")
	}

	// Second interval => second scripted draw (0 => "a").
	clk.Advance(interval)
	if got, _ := s.Next(protocols.ProtoHTTP); got.ID != "a" {
		t.Fatalf("second random pick: got %q, want %q", got.ID, "a")
	}
}

// TestRotatingSelectorNoTargetsSkips verifies that asking for a protocol with no
// configured targets returns ok=false (a benign skip), for both an absent protocol
// key and a present-but-empty target list.
func TestRotatingSelectorNoTargetsSkips(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC))
	r := rng.NewFake(rng.FakeScript{})
	byProto := map[protocols.ProtocolID][]protocols.Target{
		protocols.ProtoHTTP: httpTargets("only"),
		protocols.ProtoDNS:  {}, // present but empty
	}

	s := newRotatingSelector(clk, r, scenario.RotationSequential, time.Minute, byProto)

	if _, ok := s.Next(protocols.ProtoSSH); ok {
		t.Fatalf("absent protocol: ok=true, want false")
	}
	if _, ok := s.Next(protocols.ProtoDNS); ok {
		t.Fatalf("empty target list: ok=true, want false")
	}
	// Sanity: a configured protocol still resolves.
	if got, ok := s.Next(protocols.ProtoHTTP); !ok || got.ID != "only" {
		t.Fatalf("configured protocol: got %q ok=%v, want %q true", got.ID, ok, "only")
	}
}

// TestRotatingSelectorIntervalZeroNeverRotates verifies that a non-positive interval
// pins the selector to index 0 forever, regardless of elapsed clock time, for both
// the sequential and random strategies (random must draw nothing).
func TestRotatingSelectorIntervalZeroNeverRotates(t *testing.T) {
	cases := []struct {
		name     string
		strategy scenario.RotationStrategy
		interval time.Duration
	}{
		{"sequential/zero", scenario.RotationSequential, 0},
		{"sequential/negative", scenario.RotationSequential, -time.Second},
		{"random/zero", scenario.RotationRandom, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clk := clock.NewFake(time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC))
			// Empty Ints script: any draw would panic ("script exhausted"), proving
			// the random strategy never draws when interval <= 0.
			r := rng.NewFake(rng.FakeScript{})
			byProto := map[protocols.ProtocolID][]protocols.Target{
				protocols.ProtoHTTP: httpTargets("x0", "x1", "x2"),
			}

			s := newRotatingSelector(clk, r, tc.strategy, tc.interval, byProto)

			for i := 0; i < 4; i++ {
				if got, _ := s.Next(protocols.ProtoHTTP); got.ID != "x0" {
					t.Fatalf("call %d: got %q, want %q (never rotates)", i, got.ID, "x0")
				}
				clk.Advance(time.Hour)
			}
		})
	}
}
