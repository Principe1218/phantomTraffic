package protocols

import "testing"

func newTestTargetSet() TargetSet {
	targets := []Target{
		{ID: "web", Proto: "http", Addr: "web.internal:443"},
		{ID: "api", Proto: "http", Addr: "api.internal:8443"},
		{ID: "bastion", Proto: "ssh", Addr: "10.0.0.5:22"},
		{ID: "dns1", Proto: "dns", Addr: "10.0.0.53:53"},
	}
	// allowedDomains adds hosts permitted for redirect/link-follow that are
	// not themselves rotation targets (design §2: explicit allowed-domains).
	return NewTargetSet(targets, []string{"cdn.internal", "assets.internal"})
}

func TestTargetSet_Permits(t *testing.T) {
	ts := newTestTargetSet()
	tests := []struct {
		name string
		host string
		want bool
	}{
		{"target host with port stripped", "web.internal", true},
		{"second target host", "api.internal", true},
		{"ssh ip host", "10.0.0.5", true},
		{"dns ip host", "10.0.0.53", true},
		{"explicit allowed domain", "cdn.internal", true},
		{"second allowed domain", "assets.internal", true},
		{"case-insensitive match", "WEB.INTERNAL", true},
		{"off-allowlist host refused", "evil.example.com", false},
		{"off-allowlist metadata refused", "169.254.169.254", false},
		{"empty host refused", "", false},
		{"unlisted subdomain refused", "sub.web.internal", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ts.Permits(tt.host); got != tt.want {
				t.Fatalf("Permits(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestTargetSet_PermitsStripsPortFromInput(t *testing.T) {
	ts := newTestTargetSet()
	if !ts.Permits("web.internal:443") {
		t.Fatal("Permits must accept a host:port argument and match on host")
	}
	if !ts.Permits("10.0.0.5:22") {
		t.Fatal("Permits must accept ip:port and match on ip")
	}
}

func TestTargetSet_TargetsFor(t *testing.T) {
	ts := newTestTargetSet()
	http := ts.TargetsFor("http")
	if len(http) != 2 {
		t.Fatalf("TargetsFor(http) = %d targets, want 2", len(http))
	}
	if ts.TargetsFor("ssh")[0].ID != "bastion" {
		t.Fatalf("TargetsFor(ssh)[0].ID = %q, want bastion", ts.TargetsFor("ssh")[0].ID)
	}
	if got := ts.TargetsFor("nonexistent"); got != nil {
		t.Fatalf("TargetsFor(nonexistent) = %v, want nil", got)
	}
}

func TestTargetSet_ZeroValueDeniesAll(t *testing.T) {
	var ts TargetSet // zero value: default-deny invariant must hold
	if ts.Permits("web.internal") {
		t.Fatal("zero-value TargetSet must deny all hosts (default-deny)")
	}
}
