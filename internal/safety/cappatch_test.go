package safety

import "testing"

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }

// WithPatch overlays only the non-nil fields and returns a copy, leaving the
// receiver untouched and all unpatched fields intact.
func TestCapSpecWithPatch(t *testing.T) {
	base := CapSpec{
		PerTargetRPS:                 10,
		GlobalRPS:                    50,
		MaxConcurrentSessions:        20,
		TotalRequestBudget:           1_000_000,
		StreamingByteRateKbps:        12_000,
		ConcurrentStreams:             3,
		PerSessionMaxDurationSeconds: 1800,
		PerSessionMaxActions:         10_000,
	}

	tests := []struct {
		name  string
		patch CapPatch
		want  CapSpec
	}{
		{
			name:  "empty patch is identity",
			patch: CapPatch{},
			want:  base,
		},
		{
			name:  "patch per-target rps only",
			patch: CapPatch{PerTargetRPS: f64(5)},
			want:  func() CapSpec { c := base; c.PerTargetRPS = 5; return c }(),
		},
		{
			name:  "patch global rps and budget",
			patch: CapPatch{GlobalRPS: f64(25), TotalRequestBudget: i64(500_000)},
			want:  func() CapSpec { c := base; c.GlobalRPS = 25; c.TotalRequestBudget = 500_000; return c }(),
		},
		{
			name:  "patch all three overlay fields",
			patch: CapPatch{PerTargetRPS: f64(2), GlobalRPS: f64(8), TotalRequestBudget: i64(100)},
			want:  func() CapSpec { c := base; c.PerTargetRPS = 2; c.GlobalRPS = 8; c.TotalRequestBudget = 100; return c }(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := base.WithPatch(tc.patch)
			if got != tc.want {
				t.Fatalf("WithPatch = %+v, want %+v", got, tc.want)
			}
			// Receiver must be unchanged (value semantics, no aliasing).
			if base.PerTargetRPS != 10 || base.GlobalRPS != 50 || base.TotalRequestBudget != 1_000_000 {
				t.Fatalf("WithPatch mutated the receiver: %+v", base)
			}
		})
	}
}
