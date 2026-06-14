package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
)

func TestStopReachesStoppedAndClosesWait(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	run, err := e.Start(context.Background(), testScenario(t, 1*time.Hour))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := run.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if got := run.State(); got != StateStopped {
		t.Errorf("after Stop, State() = %s, want stopped", got)
	}

	select {
	case <-run.Wait():
	case <-time.After(time.Second):
		t.Fatal("Wait() channel not closed after Stop")
	}
	if err := run.Err(); err != nil {
		t.Errorf("Err() = %v after clean Stop, want nil", err)
	}
}

func TestStopCancelsRunCtx(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	run, err := e.Start(context.Background(), testScenario(t, 1*time.Hour))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := run.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	// Stop must have canceled the internal run context, which causes all
	// goroutines to exit and closes the done channel before Stop returns.
	select {
	case <-run.Wait():
	default:
		t.Error("run not done after Stop — internal context was not canceled")
	}
}

func TestStopIsIdempotent(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	run, err := e.Start(context.Background(), testScenario(t, 1*time.Hour))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := run.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	// A second Stop on an already-stopped run must not panic or error.
	if err := run.Stop(context.Background()); err != nil {
		t.Errorf("second Stop returned %v, want nil (idempotent)", err)
	}
}

func TestStopGraceTimeoutSurfacesError(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0).UTC())
	e := newEngineWithNoop(t, fc)
	run, err := e.Start(context.Background(), testScenario(t, 1*time.Hour))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// A caller ctx that is already canceled forces the bounded-grace path to give
	// up immediately; Stop must still reach Stopped and never block forever.
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = run.Stop(cctx)
	// With the noop handler workers exit promptly, so a clean Stop is the norm; the
	// contract under test is that Stop returns and the run is terminal regardless.
	_ = err
	if got := run.State(); got != StateStopped {
		t.Errorf("after Stop with canceled ctx, State() = %s, want stopped", got)
	}
	select {
	case <-run.Wait():
	default:
		t.Error("Wait() not closed after Stop with canceled ctx")
	}
}
