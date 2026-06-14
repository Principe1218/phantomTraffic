package persona

import (
	"context"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

func TestBuiltinsAllValid(t *testing.T) {
	bs, err := Builtins()
	if err != nil {
		t.Fatalf("Builtins: %v", err)
	}
	for _, name := range []string{"developer", "office-worker", "admin"} {
		p, ok := bs[name]
		if !ok {
			t.Fatalf("missing built-in %q", name)
		}
		if p.Mix.Len() == 0 || p.ThinkTime == nil || p.Prints == nil {
			t.Fatalf("built-in %q incompletely compiled: %+v", name, p)
		}
	}
}

func TestLookupAndDefault(t *testing.T) {
	if DefaultPersonaName != "office-worker" {
		t.Fatalf("default = %q", DefaultPersonaName)
	}
	if _, ok, err := Lookup("office-worker"); err != nil || !ok {
		t.Fatalf("lookup default: ok=%v err=%v", ok, err)
	}
	if _, ok, err := Lookup("nope"); err != nil || ok {
		t.Fatalf("lookup miss: ok=%v err=%v", ok, err)
	}
}

func TestBuiltinToSpecRunsASession(t *testing.T) {
	bs, err := Builtins()
	if err != nil {
		t.Fatal(err)
	}
	p := bs["office-worker"]
	sel := behavior.NewRoundRobinSelector(map[protocols.ProtocolID][]protocols.Target{
		protocols.ProtoHTTP: {{ID: "w", Proto: protocols.ProtoHTTP, Addr: "w:443"}},
		protocols.ProtoDNS:  {{ID: "r", Proto: protocols.ProtoDNS, Addr: "r:53"}},
	})
	deps := protocols.SessionDeps{
		Clock: clock.NewFake(time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)),
		Rand:  rng.New(1, 2),
	}
	s, err := behavior.NewSessionFactory().NewSession(context.Background(), p.ToSpec(sel), deps)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if _, err := s.Next(context.Background()); err != nil {
		t.Fatalf("Next: %v", err)
	}
}
