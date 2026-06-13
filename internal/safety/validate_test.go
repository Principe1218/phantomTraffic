package safety

import (
	"testing"
	"time"
)

func testCeiling() Ceiling {
	return Ceiling{
		PerTargetRPS:          10,
		GlobalRPS:             50,
		MaxConcurrentSessions: 20,
		TotalRequestBudget:    1_000_000,
		StreamingByteRateKbps: 12_000,
		ConcurrentStreams:     3,
		PerSessionMaxDuration: 30 * time.Minute,
		PerSessionMaxActions:  10_000,
	}
}

// violationFields extracts the Field of each violation, in order, for comparison.
func violationFields(vs []Violation) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Field)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestValidateCaps(t *testing.T) {
	ceiling := testCeiling()
	tests := []struct {
		name       string
		declared   CapSpec
		override   bool
		wantFields []string // expected Violation.Field values, in field order
	}{
		{
			name:       "all unset is valid (no violations)",
			declared:   CapSpec{},
			override:   false,
			wantFields: nil,
		},
		{
			name: "every field declared strictly below the ceiling is valid",
			declared: CapSpec{
				PerTargetRPS:                 5,
				GlobalRPS:                    25,
				MaxConcurrentSessions:        10,
				TotalRequestBudget:           500_000,
				StreamingByteRateKbps:        6_000,
				ConcurrentStreams:            2,
				PerSessionMaxDurationSeconds: 600,
				PerSessionMaxActions:         5_000,
			},
			override:   false,
			wantFields: nil,
		},
		{
			name: "every field declared exactly at the ceiling is valid (inclusive)",
			declared: CapSpec{
				PerTargetRPS:                 10,
				GlobalRPS:                    50,
				MaxConcurrentSessions:        20,
				TotalRequestBudget:           1_000_000,
				StreamingByteRateKbps:        12_000,
				ConcurrentStreams:            3,
				PerSessionMaxDurationSeconds: 1800,
				PerSessionMaxActions:         10_000,
			},
			override:   false,
			wantFields: nil,
		},
		{
			name:       "single field above the ceiling is a violation",
			declared:   CapSpec{PerTargetRPS: 11},
			override:   false,
			wantFields: []string{"per_target_rps"},
		},
		{
			name:       "above-ceiling is permitted when override is true",
			declared:   CapSpec{PerTargetRPS: 11, GlobalRPS: 100},
			override:   true,
			wantFields: nil,
		},
		{
			name:       "negative declared field is a violation even with override",
			declared:   CapSpec{PerTargetRPS: -1},
			override:   true,
			wantFields: []string{"per_target_rps"},
		},
		{
			name:       "negative int64 budget is a violation even with override",
			declared:   CapSpec{TotalRequestBudget: -5},
			override:   true,
			wantFields: []string{"total_request_budget"},
		},
		{
			name: "multiple violations are all reported, in field order",
			declared: CapSpec{
				PerTargetRPS:                 100,   // above ceiling
				GlobalRPS:                    -1,    // negative
				MaxConcurrentSessions:        50,    // above ceiling
				ConcurrentStreams:            9,     // above ceiling
				PerSessionMaxDurationSeconds: 99999, // above ceiling (1800s)
			},
			override: false,
			wantFields: []string{
				"per_target_rps",
				"global_rps",
				"max_concurrent_sessions",
				"concurrent_streams",
				"per_session_max_duration_seconds",
			},
		},
		{
			name: "with override only the negative survives as a violation",
			declared: CapSpec{
				PerTargetRPS:          100, // above ceiling -> allowed by override
				GlobalRPS:             -1,  // negative -> still a violation
				MaxConcurrentSessions: 50,  // above ceiling -> allowed by override
			},
			override:   true,
			wantFields: []string{"global_rps"},
		},
		{
			name:       "streaming byte rate above ceiling is a violation",
			declared:   CapSpec{StreamingByteRateKbps: 20_000},
			override:   false,
			wantFields: []string{"streaming_byte_rate_kbps"},
		},
		{
			name:       "per session max actions above ceiling is a violation",
			declared:   CapSpec{PerSessionMaxActions: 50_000},
			override:   false,
			wantFields: []string{"per_session_max_actions"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateCaps(tc.declared, ceiling, tc.override)
			if !equalStrings(violationFields(got), tc.wantFields) {
				t.Fatalf("ValidateCaps fields = %v, want %v (full: %+v)",
					violationFields(got), tc.wantFields, got)
			}
			// Every violation must carry a non-empty, descriptive Msg.
			for _, v := range got {
				if v.Msg == "" {
					t.Errorf("violation for field %q has empty Msg", v.Field)
				}
			}
		})
	}
}

func TestValidateCaps_ReturnsEmptyWhenValid(t *testing.T) {
	got := ValidateCaps(CapSpec{}, testCeiling(), false)
	if len(got) != 0 {
		t.Fatalf("expected no violations, got %+v", got)
	}
}
