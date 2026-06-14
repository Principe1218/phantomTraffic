package persona

import (
	"bytes"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/behavior"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"go.yaml.in/yaml/v3"
)

func TestRawPersonaStrictDecode(t *testing.T) {
	const src = `
name: tester
think_time: { kind: lognormal, mu: 1.0, sigma: 0.5, scale: 1s }
mix:
  - { protocol: http, verb: fetch-page, cause: navigation, weight: 2 }
fingerprints: default
`
	var raw RawPersona
	dec := yaml.NewDecoder(bytes.NewReader([]byte(src)))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if raw.Name != "tester" || raw.ThinkTime.Kind != "lognormal" || len(raw.Mix) != 1 {
		t.Fatalf("unexpected decode: %+v", raw)
	}
	if raw.Mix[0].Protocol != "http" || raw.Mix[0].Weight != 2 {
		t.Fatalf("unexpected mix: %+v", raw.Mix[0])
	}
}

func TestRawPersonaRejectsUnknownKey(t *testing.T) {
	const src = `
name: tester
think_tim: { kind: constant, d: 1s }
`
	var raw RawPersona
	dec := yaml.NewDecoder(bytes.NewReader([]byte(src)))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err == nil {
		t.Fatal("expected strict-decode error for typo'd key")
	}
}

func TestPersonaToSpecMapsFields(t *testing.T) {
	mix, err := behavior.NewTemplateMix([]behavior.Template{
		{Protocol: protocols.ProtoHTTP, Verb: "fetch-page", Weight: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := Persona{
		Name:      "tester",
		Mix:       mix,
		ThinkTime: behavior.Constant{D: time.Second},
		Jitter:    behavior.NoJitter{},
		Burst:     behavior.AlwaysActive{},
		TimeOfDay: behavior.FlatTimeOfDay{},
		Shape:     behavior.SessionShape{Abandon: 0.1},
		Bounds:    behavior.DefaultBranchBounds(),
	}
	sel := behavior.NewRoundRobinSelector(map[protocols.ProtocolID][]protocols.Target{
		protocols.ProtoHTTP: {{ID: "t", Proto: protocols.ProtoHTTP, Addr: "t:443"}},
	})
	spec := p.ToSpec(sel)
	if spec.Selector != sel || spec.Shape.Abandon != 0.1 || spec.ThinkTime == nil {
		t.Fatalf("ToSpec did not map fields: %+v", spec)
	}
}
