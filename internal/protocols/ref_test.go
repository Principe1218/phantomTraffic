package protocols

import "testing"

func TestRefString(t *testing.T) {
	tests := []struct {
		name string
		ref  Ref
		want string
	}{
		{"http fetch-page", Ref{Protocol: ProtoHTTP, Verb: "fetch-page"}, "http:fetch-page"},
		{"dns resolve-name", Ref{Protocol: ProtoDNS, Verb: "resolve-name"}, "dns:resolve-name"},
		{"empty verb", Ref{Protocol: ProtoSSH, Verb: ""}, "ssh:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.String(); got != tt.want {
				t.Fatalf("Ref.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Params is the open Action marker held opaquely; any Action value must be
// assignable to Params, and a nil Params must be the Plan-3 norm.
func TestParamsIsOpenActionAlias(t *testing.T) {
	var p Params
	if p != nil {
		t.Fatalf("zero Params must be nil, got %v", p)
	}
	// Compile-time: an Action is assignable to Params (alias). A concrete
	// fakeAction proves out-of-package types satisfy it without sealing.
	var a Action = fakeAction{}
	p = a
	if p.Kind() != "noop" {
		t.Fatalf("Params.Kind() = %q, want %q", p.Kind(), "noop")
	}
}

// fakeAction is a minimal Action used only to prove Params assignability.
type fakeAction struct{ BaseAction }

func (fakeAction) Kind() ActionKind { return "noop" }
func (fakeAction) Validate() error  { return nil }
