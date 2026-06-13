package scenario

import (
	"strings"
	"testing"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

func TestFieldErrorError(t *testing.T) {
	tests := []struct {
		name string
		in   FieldError
		want string
	}{
		{"simple field", FieldError{Field: "name", Msg: "must be non-empty"}, "name: must be non-empty"},
		{"indexed field", FieldError{Field: "scenarios[1].id", Msg: "duplicate id \"web\""}, "scenarios[1].id: duplicate id \"web\""},
		{"caps field", FieldError{Field: "caps.per_target_rps", Msg: "exceeds ceiling 10"}, "caps.per_target_rps: exceeds ceiling 10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Error(); got != tt.want {
				t.Fatalf("FieldError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFieldErrorClassIsConfig(t *testing.T) {
	fe := FieldError{Field: "name", Msg: "must be non-empty"}
	if got := fe.Class(); got != pterr.ClassConfig {
		t.Fatalf("FieldError.Class() = %v, want %v (ClassConfig)", got, pterr.ClassConfig)
	}
}

func TestValidationErrorsError(t *testing.T) {
	ve := ValidationErrors{
		{Field: "name", Msg: "must be non-empty"},
		{Field: "scenarios", Msg: "at least one scenario block is required"},
		{Field: "scenarios[0].protocol", Msg: "unknown protocol \"htttp\""},
	}
	got := ve.Error()
	for _, want := range []string{
		"name: must be non-empty",
		"scenarios: at least one scenario block is required",
		"scenarios[0].protocol: unknown protocol \"htttp\"",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ValidationErrors.Error() = %q, missing %q", got, want)
		}
	}
	// All three messages must be joined into one string (3 distinct fragments).
	if strings.Count(got, ": ") < 3 {
		t.Fatalf("ValidationErrors.Error() = %q, want at least 3 joined field errors", got)
	}
}

func TestValidationErrorsImplementsError(t *testing.T) {
	// Compile-time-ish assertion at runtime: a ValidationErrors must be usable
	// as the error interface so Validate can return it directly.
	var err error = ValidationErrors{{Field: "name", Msg: "x"}}
	if err.Error() == "" {
		t.Fatalf("ValidationErrors must render a non-empty error string")
	}
}
