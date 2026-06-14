package behavior

import (
	"math"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
)

func TestTemplateRef(t *testing.T) {
	tm := Template{Protocol: protocols.ProtoHTTP, Verb: "fetch-page"}
	if tm.Ref().String() != "http:fetch-page" {
		t.Fatalf("Ref = %s", tm.Ref())
	}
}

func TestTemplateMixWeightedPick(t *testing.T) {
	mix, err := NewTemplateMix([]Template{
		{Protocol: protocols.ProtoSSH, Verb: "run", Weight: 1},
		{Protocol: protocols.ProtoHTTP, Verb: "fetch-page", Weight: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	// x = Float64 * total(4): 0.1 -> 0.4 (< 1) -> ssh; 0.5 -> 2.0 -> http.
	r := rng.NewFake(rng.FakeScript{Floats: []float64{0.1, 0.5}})
	if got := mix.Pick(r); got.Protocol != protocols.ProtoSSH {
		t.Fatalf("first pick = %v, want ssh", got.Protocol)
	}
	if got := mix.Pick(r); got.Protocol != protocols.ProtoHTTP {
		t.Fatalf("second pick = %v, want http", got.Protocol)
	}
}

func TestNewTemplateMixRejectsBadWeights(t *testing.T) {
	if _, err := NewTemplateMix(nil); err == nil {
		t.Fatal("expected error for empty mix")
	}
	if _, err := NewTemplateMix([]Template{{Protocol: protocols.ProtoDNS, Verb: "query", Weight: 0}}); err == nil {
		t.Fatal("expected error for zero weight")
	}
	if _, err := NewTemplateMix([]Template{{Protocol: protocols.ProtoDNS, Verb: "query", Weight: math.NaN()}}); err == nil {
		t.Fatal("expected error for NaN weight")
	}
	if _, err := NewTemplateMix([]Template{{Protocol: protocols.ProtoDNS, Verb: "query", Weight: math.Inf(1)}}); err == nil {
		t.Fatal("expected error for +Inf weight")
	}
}
