package protocols

import (
	"context"
	"sort"
	"testing"
)

// fakeHandler is an in-test ProtocolHandler with NO real I/O. It validates an
// action via As[T], opens a trivial state, and returns a scrubbed Result plus
// a by-value Observation — exercising the whole contract surface offline.
type fakeHandler struct{ proto ProtocolID }

type fakeHandlerState struct{ closed bool }

func (fakeHandlerState) isSessionState() {}

// fakePingAction is the one action fakeHandler routes.
type fakePingAction struct {
	BaseAction
	Note string
}

func (fakePingAction) Kind() ActionKind { return "ping" }
func (fakePingAction) Validate() error  { return nil }

var _ Action = fakePingAction{}

func (h fakeHandler) ID() ProtocolID { return h.proto }

func (h fakeHandler) Capability() Capability {
	return Capability{Proto: h.proto, Actions: []ActionKind{"ping"}}
}

func (h fakeHandler) OpenState(ctx context.Context, s *Session) (SessionState, error) {
	return &fakeHandlerState{}, nil
}

func (h fakeHandler) Do(ctx context.Context, s *Session, a Action) (Result, Observation, error) {
	// Single audited cast site (design §2): wrong type returns a *RoutingError.
	pa, err := As[fakePingAction](a)
	if err != nil {
		return Result{}, Observation{}, err
	}
	res := Result{
		Protocol: h.proto,
		Action:   pa.Kind(),
		Session:  s.ID,
		Seq:      1,
		Outcome:  OutcomeSuccess,
		BytesOut: int64(len(pa.Note)),
	}
	obs := Observation{Throughput: 0, Has: 0}
	return res, obs, nil
}

func (h fakeHandler) CloseState(ctx context.Context, st SessionState) error {
	if fs, ok := st.(*fakeHandlerState); ok {
		fs.closed = true
	}
	return nil
}

var _ ProtocolHandler = fakeHandler{}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeHandler{proto: "http"}); err != nil {
		t.Fatalf("Register(http) error: %v", err)
	}
	if err := r.Register(fakeHandler{proto: "dns"}); err != nil {
		t.Fatalf("Register(dns) error: %v", err)
	}
	h, ok := r.Lookup("http")
	if !ok {
		t.Fatal("Lookup(http) must find the registered handler")
	}
	if h.ID() != "http" {
		t.Fatalf("Lookup(http).ID() = %q, want http", h.ID())
	}
	if _, ok := r.Lookup("ssh"); ok {
		t.Fatal("Lookup(ssh) must report not-found for an unregistered protocol")
	}
}

func TestRegistry_RejectsDuplicateAndEmpty(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeHandler{proto: "http"}); err != nil {
		t.Fatalf("first Register(http) error: %v", err)
	}
	if err := r.Register(fakeHandler{proto: "http"}); err == nil {
		t.Fatal("duplicate Register(http) must error")
	} else if _, ok := err.(*RoutingError); !ok {
		t.Fatalf("duplicate registration error = %T, want *RoutingError", err)
	}
	if err := r.Register(fakeHandler{proto: ""}); err == nil {
		t.Fatal("Register with empty ProtocolID must error")
	}
	if err := r.Register(nil); err == nil {
		t.Fatal("Register(nil) must error")
	}
}

func TestRegistry_HandlersEnumeratesAll(t *testing.T) {
	r := NewRegistry()
	for _, p := range []ProtocolID{"http", "dns", "ssh"} {
		if err := r.Register(fakeHandler{proto: p}); err != nil {
			t.Fatalf("Register(%s) error: %v", p, err)
		}
	}
	all := r.Handlers()
	if len(all) != 3 {
		t.Fatalf("Handlers() returned %d, want 3", len(all))
	}
	got := make([]string, 0, len(all))
	for _, h := range all {
		got = append(got, string(h.ID()))
	}
	sort.Strings(got)
	want := []string{"dns", "http", "ssh"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Handlers() ids = %v, want %v", got, want)
		}
	}
}

func TestRegistry_FakeHandlerDoRoundTrip(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeHandler{proto: "http"}); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	h, ok := r.Lookup("http")
	if !ok {
		t.Fatal("handler not found")
	}
	ctx := context.Background()
	sess := &Session{ID: "sess-roundtrip"}

	st, err := h.OpenState(ctx, sess)
	if err != nil {
		t.Fatalf("OpenState error: %v", err)
	}

	act := fakePingAction{BaseAction: BaseAction{Proto: "http", C: CauseNavigation, P: PacingShaperManaged}, Note: "hello"}
	res, obs, err := h.Do(ctx, sess, act)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if res.Outcome != OutcomeSuccess || res.Action != "ping" || res.Session != "sess-roundtrip" {
		t.Fatalf("unexpected Result: %+v", res)
	}
	if res.BytesOut != int64(len("hello")) {
		t.Fatalf("Result.BytesOut = %d, want %d", res.BytesOut, len("hello"))
	}
	if obs.Has != 0 {
		t.Fatalf("Observation.Has = %v, want 0", obs.Has)
	}

	// Routing failure: a different concrete action returns a *RoutingError.
	_, _, err = h.Do(ctx, sess, fakeReqAction{BaseAction: BaseAction{Proto: "http"}})
	if err == nil {
		t.Fatal("Do with wrong action type must return a routing error")
	}
	if _, ok := err.(*RoutingError); !ok {
		t.Fatalf("Do wrong-type error = %T, want *RoutingError", err)
	}

	if err := h.CloseState(ctx, st); err != nil {
		t.Fatalf("CloseState error: %v", err)
	}
	if fs, ok := st.(*fakeHandlerState); !ok || !fs.closed {
		t.Fatal("CloseState must mark the state closed")
	}
}
