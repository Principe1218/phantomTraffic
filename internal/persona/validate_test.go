package persona

import (
	"errors"
	"testing"
)

func validRawPersona() RawPersona {
	return RawPersona{
		Name:         "tester",
		ThinkTime:    RawDist{Kind: "lognormal", Mu: 1.0, Sigma: 0.5, Scale: "1s"},
		Jitter:       RawJitter{Kind: "proportional", Fraction: 0.1},
		Burst:        RawBurst{Active: RawDist{Kind: "exponential", Mean: "60s"}, Idle: RawDist{Kind: "constant", D: "30s"}},
		TimeOfDay:    RawCurve{}, // omitted -> flat
		Session:      RawShape{Length: RawDist{Kind: "constant", D: "30m"}, Abandon: 0.05},
		Mix:          []RawTemplate{{Protocol: "http", Verb: "fetch-page", Cause: "navigation", Pacing: "shaper-managed", Weight: 2}},
		Fingerprints: "default",
	}
}

func TestCompileValid(t *testing.T) {
	p, err := Compile(validRawPersona())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if p.Name != "tester" || p.Mix.Len() != 1 || p.ThinkTime == nil || p.Prints == nil {
		t.Fatalf("bad persona: %+v", p)
	}
}

func TestCompileErrors(t *testing.T) {
	mut := func(f func(*RawPersona)) RawPersona { r := validRawPersona(); f(&r); return r }
	tests := []struct {
		name string
		raw  RawPersona
	}{
		{"empty name", mut(func(r *RawPersona) { r.Name = "" })},
		{"unknown think kind", mut(func(r *RawPersona) { r.ThinkTime.Kind = "bogus" })},
		{"bad duration", mut(func(r *RawPersona) { r.ThinkTime = RawDist{Kind: "constant", D: "notaduration"} })},
		{"unknown protocol", mut(func(r *RawPersona) { r.Mix[0].Protocol = "gopher" })},
		{"unknown cause", mut(func(r *RawPersona) { r.Mix[0].Cause = "telepathy" })},
		{"unknown pacing", mut(func(r *RawPersona) { r.Mix[0].Pacing = "warp" })},
		{"empty verb", mut(func(r *RawPersona) { r.Mix[0].Verb = "" })},
		{"bad verb charset", mut(func(r *RawPersona) { r.Mix[0].Verb = "Fetch Page" })},
		{"zero weight", mut(func(r *RawPersona) { r.Mix[0].Weight = 0 })},
		{"empty mix", mut(func(r *RawPersona) { r.Mix = nil })},
		{"abandon out of range", mut(func(r *RawPersona) { r.Session.Abandon = 1.5 })},
		{"unknown fingerprints", mut(func(r *RawPersona) { r.Fingerprints = "exotic" })},
		{"bad curve length", mut(func(r *RawPersona) {
			r.TimeOfDay = RawCurve{Location: "UTC", Weekday: make([]float64, 23), Weekend: make([]float64, 24)}
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Compile(tt.raw); err == nil {
				t.Fatalf("expected error for %q", tt.name)
			}
		})
	}
}

func TestCompileRejectsBadDistParams(t *testing.T) {
	mut := func(f func(*RawPersona)) RawPersona { r := validRawPersona(); f(&r); return r }
	tests := []struct {
		name string
		raw  RawPersona
	}{
		{"negative constant d", mut(func(r *RawPersona) {
			r.ThinkTime = RawDist{Kind: "constant", D: "-1s"}
		})},
		{"uniform max less than min", mut(func(r *RawPersona) {
			r.ThinkTime = RawDist{Kind: "uniform", Min: "5s", Max: "1s"}
		})},
		{"negative normal stddev", mut(func(r *RawPersona) {
			r.ThinkTime = RawDist{Kind: "normal", Mean: "5s", StdDev: "-1s", Min: "0s", Max: "10s"}
		})},
		{"negative lognormal sigma", mut(func(r *RawPersona) {
			r.ThinkTime = RawDist{Kind: "lognormal", Mu: 1.0, Sigma: -0.5, Scale: "1s"}
		})},
		{"zero exponential mean", mut(func(r *RawPersona) {
			r.ThinkTime = RawDist{Kind: "exponential", Mean: "0s"}
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Compile(tt.raw); err == nil {
				t.Fatalf("expected error for %q", tt.name)
			}
		})
	}
}

func TestCompileAggregatesMultipleErrors(t *testing.T) {
	raw := validRawPersona()
	raw.Name = ""
	raw.Mix[0].Protocol = "gopher"
	_, err := Compile(raw)
	if err == nil {
		t.Fatal("expected errors")
	}
	var es Errors
	if !errors.As(err, &es) || len(es) < 2 {
		t.Fatalf("expected >=2 aggregated field errors, got %v", err)
	}
}
