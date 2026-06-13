package scenario

import (
	"strings"
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/protocols"
)

// loadFixture reads + strict-decodes a testdata scenario via Module 4's Load.
func loadFixture(t *testing.T, name string) Raw {
	t.Helper()
	raw, err := Load("testdata/" + name)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", name, err)
	}
	return raw
}

func TestValidateHappyPath(t *testing.T) {
	raw := loadFixture(t, "valid_full.yaml")
	sc, err := Validate(raw, Options{AllowInsecure: false, CapOverride: false, AgentCount: 1})
	if err != nil {
		t.Fatalf("Validate returned error on a valid fixture: %v", err)
	}

	if sc.Name != "company baseline" {
		t.Fatalf("Scenario.Name = %q, want %q", sc.Name, "company baseline")
	}
	if sc.Description != "a fully valid scenario covering all four protocols" {
		t.Fatalf("Scenario.Description = %q, unexpected", sc.Description)
	}
	if sc.AgentCount != 1 {
		t.Fatalf("Scenario.AgentCount = %d, want 1", sc.AgentCount)
	}
	if len(sc.Blocks) != 4 {
		t.Fatalf("len(Scenario.Blocks) = %d, want 4", len(sc.Blocks))
	}

	// Parsed protocols, in source order.
	wantProto := []protocols.ProtocolID{
		protocols.ProtoHTTP, protocols.ProtoDNS, protocols.ProtoSSH, protocols.ProtoStream,
	}
	for i, want := range wantProto {
		if sc.Blocks[i].Protocol != want {
			t.Fatalf("Blocks[%d].Protocol = %q, want %q", i, sc.Blocks[i].Protocol, want)
		}
	}

	// Block 0: rotation sequential, interval 300s, two typed targets.
	b0 := sc.Blocks[0]
	if b0.ID != "web" {
		t.Fatalf("Blocks[0].ID = %q, want %q", b0.ID, "web")
	}
	if b0.Rotation != RotationSequential {
		t.Fatalf("Blocks[0].Rotation = %v, want RotationSequential", b0.Rotation)
	}
	if b0.RotationInterval != 300*time.Second {
		t.Fatalf("Blocks[0].RotationInterval = %v, want 300s", b0.RotationInterval)
	}
	if len(b0.Targets) != 2 {
		t.Fatalf("len(Blocks[0].Targets) = %d, want 2", len(b0.Targets))
	}
	if b0.Targets[0].Addr != "web.company.com" || b0.Targets[0].Proto != protocols.ProtoHTTP {
		t.Fatalf("Blocks[0].Targets[0] = %+v, want addr web.company.com proto http", b0.Targets[0])
	}
	if b0.Targets[1].Addr != "api.company.com:8443" {
		t.Fatalf("Blocks[0].Targets[1].Addr = %q, want api.company.com:8443", b0.Targets[1].Addr)
	}
	if !b0.Targets[0].Cred.IsZero() {
		t.Fatalf("Blocks[0].Targets[0].Cred must be the zero CredentialRef")
	}

	// Block 1: rotation random, interval 0 (engine default), one target.
	b1 := sc.Blocks[1]
	if b1.Rotation != RotationRandom {
		t.Fatalf("Blocks[1].Rotation = %v, want RotationRandom", b1.Rotation)
	}
	if b1.RotationInterval != 0 {
		t.Fatalf("Blocks[1].RotationInterval = %v, want 0", b1.RotationInterval)
	}

	// Execution: sequential, stop-on-error true.
	if sc.Execution.Mode != ExecSequential {
		t.Fatalf("Execution.Mode = %v, want ExecSequential", sc.Execution.Mode)
	}
	if !sc.Execution.StopOnError {
		t.Fatalf("Execution.StopOnError = false, want true")
	}

	// Frozen allowlist: every listed host + allowed_domains is permitted; an
	// unlisted host is refused (default deny).
	for _, host := range []string{
		"web.company.com", "api.company.com", "ns1.company.com",
		"bastion.company.com", "media.company.com", "cdn.company.com",
	} {
		if !sc.Targets.Permits(host) {
			t.Fatalf("Targets.Permits(%q) = false, want true", host)
		}
	}
	if sc.Targets.Permits("evil.example.com") {
		t.Fatalf("Targets.Permits(%q) = true, want false (off-allowlist)", "evil.example.com")
	}

	// Effective caps populated (declared values, all under ceiling at agentCount=1).
	if sc.Caps.PerTargetRPS != 5 {
		t.Fatalf("Caps.PerTargetRPS = %v, want 5", sc.Caps.PerTargetRPS)
	}
	if sc.Caps.GlobalRPS != 25 {
		t.Fatalf("Caps.GlobalRPS = %v, want 25", sc.Caps.GlobalRPS)
	}
	if sc.Caps.MaxConcurrentSessions != 10 {
		t.Fatalf("Caps.MaxConcurrentSessions = %d, want 10", sc.Caps.MaxConcurrentSessions)
	}
	if sc.Caps.PerSessionMaxActions != 5000 {
		t.Fatalf("Caps.PerSessionMaxActions = %d, want 5000", sc.Caps.PerSessionMaxActions)
	}

	// Ceiling stored is the (divided-by-1) default ceiling.
	if sc.Ceiling.PerTargetRPS != 10 {
		t.Fatalf("Ceiling.PerTargetRPS = %v, want 10 (default, agentCount=1)", sc.Ceiling.PerTargetRPS)
	}
}

func TestValidateOmittedCapsInheritCeiling(t *testing.T) {
	// A scenario with NO caps block: every effective cap inherits the ceiling.
	raw := Raw{
		Name: "minimal",
		Scenarios: []RawBlock{
			{ID: "only", Protocol: "http", Targets: []string{"host.example.com"}},
		},
	}
	sc, err := Validate(raw, Options{AgentCount: 1})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if sc.Caps.PerTargetRPS != 10 {
		t.Fatalf("inherited Caps.PerTargetRPS = %v, want 10 (ceiling)", sc.Caps.PerTargetRPS)
	}
	if sc.Caps.GlobalRPS != 50 {
		t.Fatalf("inherited Caps.GlobalRPS = %v, want 50 (ceiling)", sc.Caps.GlobalRPS)
	}
	if sc.Caps.MaxConcurrentSessions != 20 {
		t.Fatalf("inherited Caps.MaxConcurrentSessions = %d, want 20 (ceiling)", sc.Caps.MaxConcurrentSessions)
	}
	if sc.Blocks[0].Rotation != RotationSequential {
		t.Fatalf("empty target_rotation must default to RotationSequential, got %v", sc.Blocks[0].Rotation)
	}
	if sc.Execution.Mode != ExecParallel {
		t.Fatalf("empty execution.mode must default to ExecParallel, got %v", sc.Execution.Mode)
	}
}

func TestValidateAggregatesStructuralErrors(t *testing.T) {
	raw := loadFixture(t, "invalid_structural.yaml")
	_, err := Validate(raw, Options{AgentCount: 1})
	if err == nil {
		t.Fatalf("Validate returned nil error on a structurally invalid fixture")
	}
	ve, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("Validate error is %T, want ValidationErrors", err)
	}

	gotFields := make(map[string]string, len(ve))
	for _, fe := range ve {
		gotFields[fe.Field] = fe.Msg
	}

	wantFields := []string{
		"name",                  // empty name
		"scenarios[1].id",       // duplicate "dup"
		"scenarios[1].protocol", // typo "htttp"
		"scenarios[2].targets",  // empty targets list
	}
	for _, f := range wantFields {
		if _, present := gotFields[f]; !present {
			t.Fatalf("missing aggregated FieldError for %q; got fields: %v", f, keysOf(gotFields))
		}
	}

	// Aggregation, not first-failure: all four problems reported together.
	if len(ve) < 4 {
		t.Fatalf("expected >= 4 aggregated field errors, got %d: %v", len(ve), keysOf(gotFields))
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestValidateTargetErrors(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantSub string // substring expected in the FieldError.Msg
	}{
		{"embedded creds", "user:pass@host.example.com", "embedded credentials"},
		{"embedded at only", "admin@host.example.com", "embedded credentials"},
		{"url scheme http", "http://host.example.com", "not a URL"},
		{"url scheme https", "https://host.example.com:443", "not a URL"},
		{"bad host charset underscore", "bad_host.example.com", "valid hostname or IP"},
		{"bad host charset space", "bad host.example.com", "valid hostname or IP"},
		{"label too long", strings.Repeat("a", 64) + ".example.com", "valid hostname or IP"},
		{"port zero", "host.example.com:0", "1..65535"},
		{"port too high", "host.example.com:70000", "1..65535"},
		{"port non-numeric", "host.example.com:http", "1..65535"},
		{"empty target", "", "non-empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := Raw{
				Name: "t",
				Scenarios: []RawBlock{
					{ID: "b", Protocol: "http", Targets: []string{tt.target}},
				},
			}
			_, err := Validate(raw, Options{AgentCount: 1})
			if err == nil {
				t.Fatalf("Validate accepted invalid target %q", tt.target)
			}
			ve, ok := err.(ValidationErrors)
			if !ok {
				t.Fatalf("error is %T, want ValidationErrors", err)
			}
			const wantField = "scenarios[0].targets[0]"
			var found bool
			for _, fe := range ve {
				if fe.Field == wantField && strings.Contains(fe.Msg, tt.wantSub) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("target %q: no FieldError at %q containing %q; got %v",
					tt.target, wantField, tt.wantSub, ve)
			}
		})
	}
}

func TestValidateAcceptsIPAndBracketedIPv6Targets(t *testing.T) {
	tests := []string{
		"192.0.2.10",
		"192.0.2.10:8080",
		"[2001:db8::1]:443",
	}
	for _, addr := range tests {
		t.Run(addr, func(t *testing.T) {
			raw := Raw{
				Name: "t",
				Scenarios: []RawBlock{
					{ID: "b", Protocol: "http", Targets: []string{addr}},
				},
			}
			sc, err := Validate(raw, Options{AgentCount: 1})
			if err != nil {
				t.Fatalf("Validate rejected valid target %q: %v", addr, err)
			}
			if len(sc.Blocks[0].Targets) != 1 || sc.Blocks[0].Targets[0].Addr != addr {
				t.Fatalf("target %q not parsed into Block.Targets: %+v", addr, sc.Blocks[0].Targets)
			}
		})
	}
}

func TestValidateInsecureGate(t *testing.T) {
	baseBlock := func(allowInsecure bool, reason string) RawBlock {
		return RawBlock{
			ID:                  "b",
			Protocol:            "http",
			Targets:             []string{"host.example.com"},
			AllowInsecure:       allowInsecure,
			AllowInsecureReason: reason,
		}
	}

	tests := []struct {
		name       string
		allowInsec bool   // block-level allow_insecure
		reason     string // block-level allow_insecure_reason
		optAllow   bool   // --allow-insecure invocation flag
		wantValid  bool
		wantField  string // expected FieldError.Field when invalid
		wantMsgSub string // expected substring of the FieldError.Msg when invalid
	}{
		{
			name:       "true without reason -> error",
			allowInsec: true, reason: "", optAllow: true,
			wantValid: false, wantField: "scenarios[0].allow_insecure_reason", wantMsgSub: "non-empty",
		},
		{
			name:       "true with reason but no flag -> error",
			allowInsec: true, reason: "lab-only mitm proxy", optAllow: false,
			wantValid: false, wantField: "scenarios[0].allow_insecure", wantMsgSub: "--allow-insecure",
		},
		{
			name:       "true with reason and flag -> valid",
			allowInsec: true, reason: "lab-only mitm proxy", optAllow: true,
			wantValid: true,
		},
		{
			name:       "false is always fine regardless of flag",
			allowInsec: false, reason: "", optAllow: false,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := Raw{
				Name:      "t",
				Scenarios: []RawBlock{baseBlock(tt.allowInsec, tt.reason)},
			}
			sc, err := Validate(raw, Options{AllowInsecure: tt.optAllow, AgentCount: 1})

			if tt.wantValid {
				if err != nil {
					t.Fatalf("expected valid, got error: %v", err)
				}
				if sc.Blocks[0].AllowInsecure != tt.allowInsec {
					t.Fatalf("Block.AllowInsecure = %v, want %v", sc.Blocks[0].AllowInsecure, tt.allowInsec)
				}
				if tt.allowInsec && sc.Blocks[0].AllowInsecureReason != tt.reason {
					t.Fatalf("Block.AllowInsecureReason = %q, want %q", sc.Blocks[0].AllowInsecureReason, tt.reason)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected a validation error, got nil")
			}
			ve, ok := err.(ValidationErrors)
			if !ok {
				t.Fatalf("error is %T, want ValidationErrors", err)
			}
			var found bool
			for _, fe := range ve {
				if fe.Field == tt.wantField && strings.Contains(fe.Msg, tt.wantMsgSub) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing FieldError at %q containing %q; got %v", tt.wantField, tt.wantMsgSub, ve)
			}
		})
	}
}

func TestValidateCapsOverCeiling(t *testing.T) {
	raw := loadFixture(t, "caps_over_ceiling.yaml")

	// per_target_rps 11 > ceiling 10 (agentCount=1), no override -> FieldError.
	_, err := Validate(raw, Options{AgentCount: 1, CapOverride: false})
	if err == nil {
		t.Fatalf("expected a violation for per_target_rps 11 over ceiling 10")
	}
	ve, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error is %T, want ValidationErrors", err)
	}
	var found bool
	for _, fe := range ve {
		if fe.Field == "caps.per_target_rps" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing FieldError at caps.per_target_rps; got %v", ve)
	}
}

func TestValidateCapsOverrideAllowsExceedingCeiling(t *testing.T) {
	raw := loadFixture(t, "caps_over_ceiling.yaml")
	// Same fixture, but --i-understand-this-can-dos-targets (CapOverride) is set.
	sc, err := Validate(raw, Options{AgentCount: 1, CapOverride: true})
	if err != nil {
		t.Fatalf("override should permit exceeding the ceiling, got error: %v", err)
	}
	if sc.Caps.PerTargetRPS != 11 {
		t.Fatalf("with override, effective Caps.PerTargetRPS = %v, want 11", sc.Caps.PerTargetRPS)
	}
}

func TestValidateAgentCountHalvesCeiling(t *testing.T) {
	// A declared per_target_rps of 6 is legal at agentCount=1 (ceiling 10) but a
	// violation at agentCount=2 (ceiling 10/2 = 5), proving the ceiling is
	// divided before validation.
	raw := Raw{
		Name: "agent-count",
		Caps: RawCaps{PerTargetRPS: 6},
		Scenarios: []RawBlock{
			{ID: "web", Protocol: "http", Targets: []string{"host.example.com"}},
		},
	}

	// agentCount 1: 6 <= 10 -> valid.
	if _, err := Validate(raw, Options{AgentCount: 1}); err != nil {
		t.Fatalf("per_target_rps 6 should be valid at agentCount=1, got: %v", err)
	}

	// agentCount 2: 6 > 5 -> violation at caps.per_target_rps, and the stored
	// ceiling reflects the division.
	_, err := Validate(raw, Options{AgentCount: 2})
	if err == nil {
		t.Fatalf("per_target_rps 6 should violate the halved ceiling (5) at agentCount=2")
	}
	ve, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error is %T, want ValidationErrors", err)
	}
	var found bool
	for _, fe := range ve {
		if fe.Field == "caps.per_target_rps" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing FieldError at caps.per_target_rps for halved ceiling; got %v", ve)
	}
}

func TestValidateAgentCountStoresDividedCeiling(t *testing.T) {
	// With a valid (low) cap, agentCount=2 still stores the DIVIDED ceiling on
	// the frozen Scenario for later distributed enforcement (Plan 4).
	raw := Raw{
		Name: "divided-ceiling",
		Caps: RawCaps{PerTargetRPS: 4}, // 4 <= 5 (halved) -> valid
		Scenarios: []RawBlock{
			{ID: "web", Protocol: "http", Targets: []string{"host.example.com"}},
		},
	}
	sc, err := Validate(raw, Options{AgentCount: 2})
	if err != nil {
		t.Fatalf("per_target_rps 4 should be valid at agentCount=2, got: %v", err)
	}
	if sc.Ceiling.PerTargetRPS != 5 {
		t.Fatalf("stored Ceiling.PerTargetRPS = %v, want 5 (10 / agentCount 2)", sc.Ceiling.PerTargetRPS)
	}
	if sc.Ceiling.GlobalRPS != 25 {
		t.Fatalf("stored Ceiling.GlobalRPS = %v, want 25 (50 / agentCount 2)", sc.Ceiling.GlobalRPS)
	}
}
