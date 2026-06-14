package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
)

// noopSession builds a minimal protocols.Session for handler-level tests.
func noopSession(clk clock.Clock) *protocols.Session {
	return &protocols.Session{
		ID:      protocols.SessionID("sess-noop"),
		Persona: "office-worker",
		States:  map[protocols.ProtocolID]protocols.SessionState{},
		Deps:    protocols.SessionDeps{Clock: clk},
	}
}

// noopAction is a trivial Action routed to the noop handler.
type noopAction struct {
	protocols.BaseAction
}

func (noopAction) Kind() protocols.ActionKind { return protocols.ActionKind("noop") }
func (noopAction) Validate() error            { return nil }

func newNoopAction() noopAction {
	return noopAction{BaseAction: protocols.BaseAction{
		Proto: protocols.ProtoHTTP,
		C:     protocols.CauseNavigation,
		P:     protocols.PacingShaperManaged,
	}}
}

func TestNoopHandler_ID_Capability(t *testing.T) {
	var h NoopHandler
	if got := h.ID(); got != protocols.ProtoHTTP {
		t.Fatalf("ID() = %q, want %q (registered under ProtoHTTP)", got, protocols.ProtoHTTP)
	}
	cap := h.Capability()
	if cap.Proto != protocols.ProtoHTTP {
		t.Fatalf("Capability().Proto = %q, want %q", cap.Proto, protocols.ProtoHTTP)
	}
	if len(cap.Actions) == 0 {
		t.Fatalf("Capability().Actions is empty, want >= 1 advertised action")
	}
	if cap.SupportsTLS || cap.SupportsProxyChain {
		t.Fatalf("Capability advertises transport features the noop handler does not have")
	}
}

func TestNoopHandler_Do_DeterministicSuccess(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewFake(start)
	h := NoopHandler{Latency: 7 * time.Millisecond}
	s := noopSession(clk)

	res, obs, err := h.Do(context.Background(), s, newNoopAction())
	if err != nil {
		t.Fatalf("Do returned err = %v, want nil", err)
	}
	if res.Outcome != protocols.OutcomeSuccess {
		t.Fatalf("Outcome = %v, want OutcomeSuccess", res.Outcome)
	}
	if res.Protocol != protocols.ProtoHTTP {
		t.Fatalf("Result.Protocol = %q, want %q", res.Protocol, protocols.ProtoHTTP)
	}
	if res.Latency != 7*time.Millisecond {
		t.Fatalf("Latency = %v, want 7ms (the configured Latency)", res.Latency)
	}
	if res.BytesIn != 0 || res.BytesOut != 0 {
		t.Fatalf("bytes = (%d,%d), want (0,0) — the noop handler is zero-byte", res.BytesIn, res.BytesOut)
	}
	if !res.StartedAt.Equal(start) {
		t.Fatalf("StartedAt = %v, want %v (the injected clock's Now at entry)", res.StartedAt, start)
	}
	if obs.Has != 0 {
		t.Fatalf("Observation.Has = %v, want 0 (empty observation)", obs.Has)
	}
	// Determinism: a second call on the same (un-advanced) clock is byte-identical.
	res2, _, _ := h.Do(context.Background(), s, newNoopAction())
	if res2.Latency != res.Latency || !res2.StartedAt.Equal(res.StartedAt) {
		t.Fatalf("second Do diverged: %+v vs %+v", res2, res)
	}
}

func TestNoopHandler_Do_FailMode(t *testing.T) {
	clk := clock.NewFake(time.Unix(0, 0).UTC())
	h := NoopHandler{Fail: true}
	res, _, err := h.Do(context.Background(), noopSession(clk), newNoopAction())
	if err != nil {
		t.Fatalf("Do(Fail) err = %v, want nil (a failure Outcome, not a Go error)", err)
	}
	if res.Outcome != protocols.OutcomeFailure {
		t.Fatalf("Outcome = %v, want OutcomeFailure", res.Outcome)
	}
}

func TestNoopHandler_Do_HonorsContextCancellation(t *testing.T) {
	clk := clock.NewFake(time.Unix(0, 0).UTC())
	h := NoopHandler{Latency: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled before entry

	_, _, err := h.Do(ctx, noopSession(clk), newNoopAction())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do err = %v, want context.Canceled when ctx is already done", err)
	}
}

func TestNoopHandler_Do_PanicMode(t *testing.T) {
	clk := clock.NewFake(time.Unix(0, 0).UTC())
	h := NoopHandler{Panic: true}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Do(Panic) did not panic; want a panic so the worker shim records OutcomePanicked")
		}
	}()
	_, _, _ = h.Do(context.Background(), noopSession(clk), newNoopAction())
}

func TestNoopHandler_OpenCloseState(t *testing.T) {
	clk := clock.NewFake(time.Unix(0, 0).UTC())
	var h NoopHandler
	st, err := h.OpenState(context.Background(), noopSession(clk))
	if err != nil {
		t.Fatalf("OpenState err = %v, want nil", err)
	}
	if err := h.CloseState(context.Background(), st); err != nil {
		t.Fatalf("CloseState err = %v, want nil", err)
	}
}
