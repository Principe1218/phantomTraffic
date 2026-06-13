package safety

import (
	"testing"
	"time"
)

func TestDefaultCeiling_D2Values(t *testing.T) {
	got := DefaultCeiling()
	want := Ceiling{
		PerTargetRPS:          10,
		GlobalRPS:             50,
		MaxConcurrentSessions: 20,
		TotalRequestBudget:    1_000_000,
		StreamingByteRateKbps: 12_000,
		ConcurrentStreams:     3,
		PerSessionMaxDuration: 30 * time.Minute,
		PerSessionMaxActions:  10_000,
	}
	if got != want {
		t.Fatalf("DefaultCeiling() = %+v, want %+v", got, want)
	}
}

func TestCeiling_DividedBy(t *testing.T) {
	base := DefaultCeiling()
	tests := []struct {
		name       string
		agentCount int
		want       Ceiling
	}{
		{
			name:       "agentCount 1 is identity",
			agentCount: 1,
			want:       DefaultCeiling(),
		},
		{
			name:       "agentCount 2 halves the aggregate caps only",
			agentCount: 2,
			want: Ceiling{
				PerTargetRPS:          5,       // 10 / 2
				GlobalRPS:             25,      // 50 / 2
				MaxConcurrentSessions: 10,      // 20 / 2
				TotalRequestBudget:    500_000, // 1_000_000 / 2
				StreamingByteRateKbps: 6_000,   // 12_000 / 2
				ConcurrentStreams:     1,       // 3 / 2 -> floor 1 (integer division)
				PerSessionMaxDuration: 30 * time.Minute,
				PerSessionMaxActions:  10_000,
			},
		},
		{
			name:       "agentCount larger than an integer cap floors that cap at 1",
			agentCount: 100,
			want: Ceiling{
				PerTargetRPS:          0.1,    // 10 / 100
				GlobalRPS:             0.5,    // 50 / 100
				MaxConcurrentSessions: 1,      // 20 / 100 -> 0, floored to 1
				TotalRequestBudget:    10_000, // 1_000_000 / 100
				StreamingByteRateKbps: 120,    // 12_000 / 100
				ConcurrentStreams:     1,      // 3 / 100 -> 0, floored to 1
				PerSessionMaxDuration: 30 * time.Minute,
				PerSessionMaxActions:  10_000,
			},
		},
		{
			name:       "agentCount 0 treated as 1 (identity)",
			agentCount: 0,
			want:       DefaultCeiling(),
		},
		{
			name:       "negative agentCount treated as 1 (identity)",
			agentCount: -7,
			want:       DefaultCeiling(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := base.DividedBy(tc.agentCount)
			if got != tc.want {
				t.Fatalf("DividedBy(%d) = %+v, want %+v", tc.agentCount, got, tc.want)
			}
			// per-session fields must never be divided
			if got.PerSessionMaxDuration != base.PerSessionMaxDuration {
				t.Errorf("PerSessionMaxDuration changed: got %v, base %v", got.PerSessionMaxDuration, base.PerSessionMaxDuration)
			}
			if got.PerSessionMaxActions != base.PerSessionMaxActions {
				t.Errorf("PerSessionMaxActions changed: got %d, base %d", got.PerSessionMaxActions, base.PerSessionMaxActions)
			}
		})
	}
}

func TestCeiling_DividedBy_DoesNotMutateReceiver(t *testing.T) {
	base := DefaultCeiling()
	snapshot := base
	_ = base.DividedBy(4)
	if base != snapshot {
		t.Fatalf("DividedBy mutated its value receiver: got %+v, want %+v", base, snapshot)
	}
}
