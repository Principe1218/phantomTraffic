package safety

import (
	"testing"
	"time"
)

func TestCapSpec_Effective(t *testing.T) {
	ceiling := Ceiling{
		PerTargetRPS:          10,
		GlobalRPS:             50,
		MaxConcurrentSessions: 20,
		TotalRequestBudget:    1_000_000,
		StreamingByteRateKbps: 12_000,
		ConcurrentStreams:     3,
		PerSessionMaxDuration: 30 * time.Minute,
		PerSessionMaxActions:  10_000,
	}
	tests := []struct {
		name     string
		declared CapSpec
		want     CapSpec
	}{
		{
			name:     "all fields unset inherit the entire ceiling",
			declared: CapSpec{},
			want: CapSpec{
				PerTargetRPS:                 10,
				GlobalRPS:                    50,
				MaxConcurrentSessions:        20,
				TotalRequestBudget:           1_000_000,
				StreamingByteRateKbps:        12_000,
				ConcurrentStreams:            3,
				PerSessionMaxDurationSeconds: 1800, // 30m expressed in whole seconds
				PerSessionMaxActions:         10_000,
			},
		},
		{
			name: "all fields set are preserved unchanged",
			declared: CapSpec{
				PerTargetRPS:                 4,
				GlobalRPS:                    20,
				MaxConcurrentSessions:        8,
				TotalRequestBudget:           250_000,
				StreamingByteRateKbps:        6_000,
				ConcurrentStreams:            2,
				PerSessionMaxDurationSeconds: 600,
				PerSessionMaxActions:         5_000,
			},
			want: CapSpec{
				PerTargetRPS:                 4,
				GlobalRPS:                    20,
				MaxConcurrentSessions:        8,
				TotalRequestBudget:           250_000,
				StreamingByteRateKbps:        6_000,
				ConcurrentStreams:            2,
				PerSessionMaxDurationSeconds: 600,
				PerSessionMaxActions:         5_000,
			},
		},
		{
			name: "mixed: only the zero fields inherit, set fields stay",
			declared: CapSpec{
				PerTargetRPS:          4,     // set
				MaxConcurrentSessions: 8,     // set
				PerSessionMaxActions:  5_000, // set
				// the rest left zero -> inherit
			},
			want: CapSpec{
				PerTargetRPS:                 4,         // kept
				GlobalRPS:                    50,        // inherited
				MaxConcurrentSessions:        8,         // kept
				TotalRequestBudget:           1_000_000, // inherited
				StreamingByteRateKbps:        12_000,    // inherited
				ConcurrentStreams:            3,         // inherited
				PerSessionMaxDurationSeconds: 1800,      // inherited (30m -> 1800s)
				PerSessionMaxActions:         5_000,     // kept
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.declared.Effective(ceiling)
			if got != tc.want {
				t.Fatalf("Effective() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCapSpec_Effective_DoesNotMutateReceiver(t *testing.T) {
	declared := CapSpec{PerTargetRPS: 4}
	snapshot := declared
	_ = declared.Effective(DefaultCeiling())
	if declared != snapshot {
		t.Fatalf("Effective mutated its value receiver: got %+v, want %+v", declared, snapshot)
	}
}
