package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
)

func TestGateOpenLogicAllSources(t *testing.T) {
	base := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		ops    func(g *gate)
		isOpen bool
	}{
		{
			name:   "fresh gate is open",
			ops:    func(g *gate) {},
			isOpen: true,
		},
		{
			name:   "operator pause closes the gate",
			ops:    func(g *gate) { g.pause(gateOperator) },
			isOpen: false,
		},
		{
			name:   "schedule pause closes the gate",
			ops:    func(g *gate) { g.pause(gateSchedule) },
			isOpen: false,
		},
		{
			name: "still closed while any source paused",
			ops: func(g *gate) {
				g.pause(gateOperator)
				g.pause(gateSchedule)
				g.resume(gateOperator) // schedule still paused
			},
			isOpen: false,
		},
		{
			name: "open only when all sources resumed",
			ops: func(g *gate) {
				g.pause(gateOperator)
				g.pause(gateSchedule)
				g.resume(gateOperator)
				g.resume(gateSchedule)
			},
			isOpen: true,
		},
		{
			name: "double pause is idempotent: one resume reopens",
			ops: func(g *gate) {
				g.pause(gateOperator)
				g.pause(gateOperator)
				g.resume(gateOperator)
			},
			isOpen: true,
		},
		{
			name: "resume of an already-open source is a no-op",
			ops: func(g *gate) {
				g.resume(gateSchedule)
			},
			isOpen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clk := clock.NewFake(base)
			g := newGate(clk)
			tt.ops(g)
			if got := g.isOpen(); got != tt.isOpen {
				t.Fatalf("isOpen() = %v, want %v", got, tt.isOpen)
			}
		})
	}
}

func TestGateWaitOpenReturnsImmediately(t *testing.T) {
	base := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	clk := clock.NewFake(base)
	g := newGate(clk)

	if err := g.wait(context.Background()); err != nil {
		t.Fatalf("wait() on open gate = %v, want nil", err)
	}
}

func TestGateWaitBlocksUntilAllSourcesResume(t *testing.T) {
	base := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	clk := clock.NewFake(base)
	g := newGate(clk)

	g.pause(gateOperator)
	g.pause(gateSchedule)

	done := make(chan error, 1)
	go func() { done <- g.wait(context.Background()) }()

	// Let the waiter park on the cond.
	time.Sleep(20 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("wait() returned while a source was still paused")
	default:
	}

	// Reopen one source: still paused, waiter must stay blocked.
	g.resume(gateOperator)
	time.Sleep(20 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("wait() returned while gateSchedule was still paused")
	default:
	}

	// Reopen the last source: waiter unblocks with nil.
	g.resume(gateSchedule)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("wait() = %v, want nil after all sources resumed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("wait() did not return after all sources resumed")
	}
}

func TestGateWaitCancelable(t *testing.T) {
	base := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	clk := clock.NewFake(base)
	g := newGate(clk)

	g.pause(gateOperator)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.wait(ctx) }()

	// Let the waiter park on the cond, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("wait() = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("wait() did not return after ctx cancel")
	}
}

func TestGatePausedTotalAccumulatesOperatorPause(t *testing.T) {
	base := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	clk := clock.NewFake(base)
	g := newGate(clk)

	if got := g.pausedTotal(); got != 0 {
		t.Fatalf("pausedTotal() on fresh gate = %v, want 0", got)
	}

	// First operator pause: 5 minutes.
	g.pause(gateOperator)
	clk.Advance(5 * time.Minute)
	g.resume(gateOperator)
	if got := g.pausedTotal(); got != 5*time.Minute {
		t.Fatalf("pausedTotal() after first pause = %v, want 5m", got)
	}

	// Time passes while open: does not count.
	clk.Advance(10 * time.Minute)
	if got := g.pausedTotal(); got != 5*time.Minute {
		t.Fatalf("pausedTotal() while open = %v, want 5m", got)
	}

	// Second operator pause: 3 minutes, accumulates onto the first.
	g.pause(gateOperator)
	clk.Advance(3 * time.Minute)
	g.resume(gateOperator)
	if got := g.pausedTotal(); got != 8*time.Minute {
		t.Fatalf("pausedTotal() after second pause = %v, want 8m", got)
	}
}

func TestGatePausedTotalIgnoresSchedulePause(t *testing.T) {
	base := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	clk := clock.NewFake(base)
	g := newGate(clk)

	// A schedule off-window contributes nothing to operator timer-shift budget.
	g.pause(gateSchedule)
	clk.Advance(2 * time.Hour)
	g.resume(gateSchedule)
	if got := g.pausedTotal(); got != 0 {
		t.Fatalf("pausedTotal() after schedule pause = %v, want 0", got)
	}
}

func TestGatePausedTotalIncludesInProgressOperatorPause(t *testing.T) {
	base := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	clk := clock.NewFake(base)
	g := newGate(clk)

	g.pause(gateOperator)
	clk.Advance(90 * time.Second)
	// Still paused: the live, in-progress interval is included.
	if got := g.pausedTotal(); got != 90*time.Second {
		t.Fatalf("pausedTotal() mid-pause = %v, want 90s", got)
	}
}
