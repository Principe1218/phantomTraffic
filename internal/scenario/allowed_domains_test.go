package scenario

import "testing"

// TestValidateRejectsMalformedAllowedDomain locks the rule that allowed_domains
// entries are host-validated (design §5.2), so a malformed entry is a ClassConfig
// FieldError rather than a silently dead, never-matching allowlist row.
func TestValidateRejectsMalformedAllowedDomain(t *testing.T) {
	raw := Raw{
		Name:           "t",
		AllowedDomains: []string{"ok.example.com", "bad host!@#"},
		Scenarios: []RawBlock{
			{ID: "b", Protocol: "http", Targets: []string{"host.example.com"}},
		},
	}
	_, err := Validate(raw, Options{AgentCount: 1})
	assertFieldError(t, requireValidationErrors(t, err), "allowed_domains[1]", "")
}

// TestValidateAcceptsValidAllowedDomains confirms well-formed allowed_domains
// pass and are admitted to the frozen allowlist.
func TestValidateAcceptsValidAllowedDomains(t *testing.T) {
	raw := Raw{
		Name:           "t",
		AllowedDomains: []string{"cdn.example.com", "static.example.com"},
		Scenarios: []RawBlock{
			{ID: "b", Protocol: "http", Targets: []string{"host.example.com"}},
		},
	}
	sc, err := Validate(raw, Options{AgentCount: 1})
	if err != nil {
		t.Fatalf("Validate rejected valid allowed_domains: %v", err)
	}
	if !sc.Targets.Permits("cdn.example.com") {
		t.Fatalf("allowed_domains entry cdn.example.com not in the frozen allowlist")
	}
}
