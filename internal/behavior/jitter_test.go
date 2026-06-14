package behavior

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

func TestNoJitterIsIdentity(t *testing.T) {
	r := rng.NewFake(rng.FakeScript{})
	if got := (NoJitter{}).Jitter(100*time.Millisecond, r); got != 100*time.Millisecond {
		t.Fatalf("NoJitter changed the duration: got %v", got)
	}
	if (NoJitter{}).Name() != "none" {
		t.Fatalf("NoJitter.Name() = %q", (NoJitter{}).Name())
	}
}

func TestProportionalJitter(t *testing.T) {
	tests := []struct {
		name     string
		fraction float64
		float    float64
		base     time.Duration
		want     time.Duration
	}{
		{"midpoint draw is identity", 0.1, 0.5, 1000 * time.Millisecond, 1000 * time.Millisecond},
		{"low draw shrinks", 0.1, 0.0, 1000 * time.Millisecond, 900 * time.Millisecond},
		{"zero fraction is identity", 0.0, 0.0, 1000 * time.Millisecond, 1000 * time.Millisecond},
		{"fraction over one is clamped to one", 5.0, 0.0, 1000 * time.Millisecond, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := rng.NewFake(rng.FakeScript{Floats: []float64{tt.float}})
			got := ProportionalJitter{Fraction: tt.fraction}.Jitter(tt.base, r)
			if got != tt.want {
				t.Fatalf("Jitter = %v, want %v", got, tt.want)
			}
		})
	}
}
