package scenario

import (
	"bytes"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestRawScenarioDecodesPersonaFields(t *testing.T) {
	const src = `
name: office hours
scenarios:
  - id: web
    protocol: http
    targets: ["web.example.com:443"]
    persona: developer
personas:
  - name: custom
    think_time: { kind: constant, d: 1s }
    mix:
      - { protocol: http, verb: fetch-page, cause: navigation, weight: 1 }
    fingerprints: default
`
	var raw Raw
	dec := yaml.NewDecoder(bytes.NewReader([]byte(src)))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if raw.Scenarios[0].Persona != "developer" {
		t.Fatalf("block persona = %q, want developer", raw.Scenarios[0].Persona)
	}
	if len(raw.Personas) != 1 || raw.Personas[0].Name != "custom" {
		t.Fatalf("custom personas = %+v", raw.Personas)
	}
}
