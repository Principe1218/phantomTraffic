package protocols

// Registry is the single, auditable list of which ProtocolHandlers are loaded
// (design §7 supply-chain). Registration is EXPLICIT at wiring time — there is
// NO init() magic and no package-level global registry — so one place in the
// codebase enumerates exactly what the binary can speak. After wiring the
// registry is read-only on the hot path, so concurrent Lookup/Handlers from
// many vuser goroutines are race-free.
type Registry struct {
	byID map[ProtocolID]ProtocolHandler
}

// NewRegistry returns an empty registry ready for explicit Register calls.
func NewRegistry() *Registry {
	return &Registry{byID: make(map[ProtocolID]ProtocolHandler)}
}

// Register adds a handler under its own ID(). It rejects a nil handler, an
// empty ProtocolID, and a duplicate registration with a *RoutingError
// (pterr.ClassPermanent) so wiring mistakes fail loudly at startup rather than
// silently shadowing a protocol. Call this only during wiring, before Start.
func (r *Registry) Register(h ProtocolHandler) error {
	if h == nil {
		return &RoutingError{Msg: "cannot register a nil ProtocolHandler"}
	}
	id := h.ID()
	if id == "" {
		return &RoutingError{Msg: "cannot register a handler with an empty ProtocolID"}
	}
	if _, exists := r.byID[id]; exists {
		return &RoutingError{Proto: id, Msg: "a handler is already registered for this protocol"}
	}
	r.byID[id] = h
	return nil
}

// Lookup returns the handler registered for id (ok==false if none). The engine
// routes each PlannedAction.Ref.Protocol through this single resolution point.
func (r *Registry) Lookup(id ProtocolID) (ProtocolHandler, bool) {
	h, ok := r.byID[id]
	return h, ok
}

// Handlers returns every registered handler (order unspecified). This is the
// enumerate-all used for capability discovery and config validation at wiring
// time, and is one of the two exhaustiveness mechanisms (design §2).
func (r *Registry) Handlers() []ProtocolHandler {
	out := make([]ProtocolHandler, 0, len(r.byID))
	for _, h := range r.byID {
		out = append(out, h)
	}
	return out
}
