package scenario

import (
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/persona"
)

func baseRaw() Raw {
	return Raw{
		Name:      "test",
		Execution: RawExecution{Mode: "parallel"},
		Scenarios: []RawBlock{
			{ID: "b1", Protocol: "http", Targets: []string{"web.example.com:443"}, TargetRotation: "sequential"},
		},
	}
}

func TestValidateDefaultsPersona(t *testing.T) {
	sc, err := Validate(baseRaw(), Options{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if sc.Blocks[0].Persona.Name != persona.DefaultPersonaName {
		t.Fatalf("expected default persona %q, got %q", persona.DefaultPersonaName, sc.Blocks[0].Persona.Name)
	}
}

func TestValidateResolvesNamedBuiltin(t *testing.T) {
	raw := baseRaw()
	raw.Scenarios[0].Persona = "developer"
	sc, err := Validate(raw, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if sc.Blocks[0].Persona.Name != "developer" {
		t.Fatalf("persona = %q, want developer", sc.Blocks[0].Persona.Name)
	}
}

func TestValidateRejectsUnknownPersona(t *testing.T) {
	raw := baseRaw()
	raw.Scenarios[0].Persona = "ghost"
	if _, err := Validate(raw, Options{}); err == nil {
		t.Fatal("expected error for unknown persona")
	}
}

func TestValidateCompilesAndUsesCustomPersona(t *testing.T) {
	raw := baseRaw()
	raw.Personas = []persona.RawPersona{{
		Name:         "tester",
		ThinkTime:    persona.RawDist{Kind: "constant", D: "1s"},
		Mix:          []persona.RawTemplate{{Protocol: "http", Verb: "fetch-page", Cause: "navigation", Weight: 1}},
		Fingerprints: "default",
	}}
	raw.Scenarios[0].Persona = "tester"
	sc, err := Validate(raw, Options{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if sc.Blocks[0].Persona.Name != "tester" {
		t.Fatalf("persona = %q, want tester", sc.Blocks[0].Persona.Name)
	}
}

func TestValidateRejectsInvalidCustomPersona(t *testing.T) {
	raw := baseRaw()
	raw.Personas = []persona.RawPersona{{Name: "", Mix: nil}} // empty name + empty mix
	if _, err := Validate(raw, Options{}); err == nil {
		t.Fatal("expected error for invalid custom persona")
	}
}
