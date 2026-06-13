package protocols

import "testing"

// fakeState proves a concrete type can satisfy the opaque SessionState marker
// without exposing its internals to the engine/behavior layers.
type fakeState struct{ open bool }

func (fakeState) isSessionState() {}

var _ SessionState = fakeState{}

func TestSessionState_MarkerIsOpaque(t *testing.T) {
	var s SessionState = fakeState{open: true}
	// The only thing callers can do with a SessionState is hold it and hand it
	// back to CloseState — it exposes no fields. A round-trip type assertion is
	// allowed inside the owning handler only.
	if _, ok := s.(fakeState); !ok {
		t.Fatal("owning handler must be able to recover its own SessionState")
	}
}

func TestCapability_DescribesProtocol(t *testing.T) {
	cap := Capability{
		Proto:              "http",
		Actions:            []ActionKind{"request", "fetch-page", "follow-link"},
		SupportsTLS:        true,
		SupportsProxyChain: false,
		TransportModes:     []string{"h1", "h2"},
	}
	if cap.Proto != "http" {
		t.Fatalf("Capability.Proto = %q, want http", cap.Proto)
	}
	if len(cap.Actions) != 3 || !cap.SupportsTLS {
		t.Fatalf("Capability not carrying its fields: %+v", cap)
	}
}

func TestSession_CarriesAllowlistAndStates(t *testing.T) {
	ts := NewTargetSet([]Target{{ID: "web", Proto: "http", Addr: "web.internal:443"}}, nil)
	s := Session{
		ID:      "sess-1",
		Persona: "developer",
		Targets: ts,
		States:  map[ProtocolID]SessionState{"http": fakeState{open: true}},
	}
	if !s.Targets.Permits("web.internal") {
		t.Fatal("Session must carry the frozen allowlist")
	}
	if _, ok := s.States["http"]; !ok {
		t.Fatal("Session must hold lazily-opened per-protocol states")
	}
	if s.Persona != "developer" {
		t.Fatalf("Session.Persona = %q, want developer", s.Persona)
	}
}
