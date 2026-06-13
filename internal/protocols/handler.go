package protocols

import "context"

// ProtocolHandler speaks one protocol's wire and exposes its action
// vocabulary (design §2). A handler is STATELESS across sessions — all
// per-session mutable state lives in the SessionState it returns from
// OpenState. It is safe for concurrent use by DISTINCT sessions; one
// protocol's SessionState within one vuser is driven by that vuser's single
// goroutine (the engine guarantees this; see design §2, §5). Every method is
// context-first (AGENTS.md §8.1) and Observation is returned BY VALUE.
type ProtocolHandler interface {
	ID() ProtocolID
	Capability() Capability // pure, no I/O — discovery/validation
	OpenState(ctx context.Context, s *Session) (SessionState, error)
	Do(ctx context.Context, s *Session, a Action) (Result, Observation, error)
	CloseState(ctx context.Context, st SessionState) error // idempotent; always called (defer)
}

// SessionState is the opaque per-(vuser, protocol) handle (design §2). The
// unexported marker method makes it un-forgeable outside the package that
// defines a concrete state; engine and behavior can only hold it and hand it
// back to CloseState — they never read its internals.
type SessionState interface{ isSessionState() } // NOSONAR — marker interface; unexported method is the intentional un-forgeable seal

// Capability is the pure, no-I/O description a handler advertises for
// discovery and config validation (design §2). It lists the supported
// action kinds and the transport dimensions (TLS, proxy/jump chains, and
// transport modes such as h1/h2 for HTTP or do53/doh/dot for DNS).
type Capability struct {
	Proto              ProtocolID
	Actions            []ActionKind
	SupportsTLS        bool
	SupportsProxyChain bool
	TransportModes     []string
}
