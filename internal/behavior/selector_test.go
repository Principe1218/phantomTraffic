package behavior

import (
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
)

func TestRoundRobinSelectorCycles(t *testing.T) {
	a := protocols.Target{ID: "a", Proto: protocols.ProtoHTTP, Addr: "a:443"}
	b := protocols.Target{ID: "b", Proto: protocols.ProtoHTTP, Addr: "b:443"}
	sel := NewRoundRobinSelector(map[protocols.ProtocolID][]protocols.Target{
		protocols.ProtoHTTP: {a, b},
	})
	want := []string{"a", "b", "a"}
	for i, w := range want {
		got, ok := sel.Next(protocols.ProtoHTTP)
		if !ok || got.ID != w {
			t.Fatalf("Next #%d = (%s,%v), want %s", i, got.ID, ok, w)
		}
	}
	// A protocol with no targets reports ok=false.
	if _, ok := sel.Next(protocols.ProtoDNS); ok {
		t.Fatal("expected ok=false for a protocol with no targets")
	}
}
