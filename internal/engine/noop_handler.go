package engine

import (
	"context"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
)

// NoopHandler is a dependency-free protocols.ProtocolHandler used as the
// Plan-4 integration fixture and the phantom-run smoke target. It performs no
// network I/O, reads no secrets, and returns a deterministic Result whose
// Latency is the injected (configurable) value, timestamped against the
// session's injected clock. It is registered under protocols.ProtoHTTP so a
// normal validated scenario (protocol: http) routes to it with no enum change.
//
// The three fields select the outcome for tests: the zero value is a
// deterministic success; Fail yields OutcomeFailure (no Go error — a recorded
// failure outcome); Panic panics inside Do so the worker's recover() shim
// records OutcomePanicked and the run survives (resilience design §7.2).
type NoopHandler struct {
	Latency time.Duration
	Fail    bool
	Panic   bool
}

// noopActionKind is the single action the noop handler advertises and accepts.
const noopActionKind protocols.ActionKind = "noop"

// ID returns ProtoHTTP: the handler is deliberately registered under the HTTP
// protocol id so a validated `protocol: http` scenario routes to it unchanged.
func (NoopHandler) ID() protocols.ProtocolID { return protocols.ProtoHTTP }

// Capability advertises a single noop action and no transport features.
func (NoopHandler) Capability() protocols.Capability {
	return protocols.Capability{
		Proto:   protocols.ProtoHTTP,
		Actions: []protocols.ActionKind{noopActionKind},
	}
}

// OpenState allocates no resources; the noop handler is stateless.
func (NoopHandler) OpenState(_ context.Context, _ *protocols.Session) (protocols.SessionState, error) {
	return nil, nil
}

// Do returns a deterministic, zero-byte Result. It honors ctx cancellation
// (returns ctx.Err() before doing any work) and never touches the wire. Latency
// is the configured value verbatim; StartedAt is the injected clock's Now at
// entry so the result is reproducible under a fake clock.
func (h NoopHandler) Do(ctx context.Context, s *protocols.Session, a protocols.Action) (protocols.Result, protocols.Observation, error) {
	if err := ctx.Err(); err != nil {
		return protocols.Result{}, protocols.Observation{}, err
	}
	if h.Panic {
		panic("noop handler: intentional panic for panic-isolation test")
	}

	outcome := protocols.OutcomeSuccess
	if h.Fail {
		outcome = protocols.OutcomeFailure
	}

	res := protocols.Result{
		Protocol:  protocols.ProtoHTTP,
		Action:    noopActionKind,
		Session:   s.ID,
		Outcome:   outcome,
		StartedAt: s.Deps.Clock.Now(),
		Latency:   h.Latency,
	}
	return res, protocols.Observation{}, nil
}

// CloseState is a no-op; OpenState allocated nothing.
func (NoopHandler) CloseState(_ context.Context, _ protocols.SessionState) error { return nil }
