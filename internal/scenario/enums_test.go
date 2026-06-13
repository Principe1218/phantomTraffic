package scenario

import "testing"

func TestRotationStrategyString(t *testing.T) {
	tests := []struct {
		name string
		in   RotationStrategy
		want string
	}{
		{"sequential is the zero value", RotationSequential, "sequential"},
		{"random", RotationRandom, "random"},
		{"out-of-range falls back to unknown", RotationStrategy(200), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Fatalf("RotationStrategy(%d).String() = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExecutionModeString(t *testing.T) {
	tests := []struct {
		name string
		in   ExecutionMode
		want string
	}{
		{"parallel is the zero value", ExecParallel, "parallel"},
		{"sequential", ExecSequential, "sequential"},
		{"out-of-range falls back to unknown", ExecutionMode(200), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Fatalf("ExecutionMode(%d).String() = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEnumIotaOrderIsStable(t *testing.T) {
	// The zero value IS the default for an empty YAML field; lock the order.
	if RotationSequential != 0 || RotationRandom != 1 {
		t.Fatalf("RotationStrategy iota changed: sequential=%d random=%d",
			RotationSequential, RotationRandom)
	}
	if ExecParallel != 0 || ExecSequential != 1 {
		t.Fatalf("ExecutionMode iota changed: parallel=%d sequential=%d",
			ExecParallel, ExecSequential)
	}
}
