package pterr

import "testing"

func TestClassString(t *testing.T) {
	tests := []struct {
		name string
		in   Class
		want string
	}{
		{"transient", ClassTransient, "transient"},
		{"permanent", ClassPermanent, "permanent"},
		{"config", ClassConfig, "config"},
		{"safety", ClassSafety, "safety"},
		{"unknown", ClassUnknown, "unknown"},
		{"out-of-range falls back to unknown label", Class(200), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Fatalf("Class(%d).String() = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestClassValuesAreStable(t *testing.T) {
	// The wire/stats label ordering is load-bearing (Result.ErrClass); lock it.
	if ClassTransient != 0 || ClassPermanent != 1 || ClassConfig != 2 || ClassSafety != 3 || ClassUnknown != 4 {
		t.Fatalf("Class iota order changed: transient=%d permanent=%d config=%d safety=%d unknown=%d",
			ClassTransient, ClassPermanent, ClassConfig, ClassSafety, ClassUnknown)
	}
}
