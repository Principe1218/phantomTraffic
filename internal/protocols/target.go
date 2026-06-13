package protocols

import (
	"strings"

	"github.com/Principe1218/phantomTraffic/internal/secret"
)

// Target carries an endpoint + a CredentialRef (NEVER secret bytes) + an
// optional proxy/jump chain (SSH bastions). Safe to serialize to YAML and
// stream to the UI (design §2). The credential is an opaque handle resolved
// lazily inside the handler via secret.CredentialSource (AGENTS.md §3.1).
type Target struct {
	ID    string
	Proto ProtocolID
	Addr  string               // host[:port] — validated upstream; never a raw user string into a dialer
	Cred  secret.CredentialRef // opaque handle; resolved lazily inside the handler
	Proxy []Target             // ordered SSH ProxyJump chain (each with its own Cred); nil if none
}

// TargetSet is the AUTHORITATIVE navigation allowlist (design §2, §7). Every
// wire request — including HTTP redirects and discovered follow-link hops —
// MUST resolve to a host in this set (or an explicit allowed-domains entry).
// Off-list navigation is REFUSED, not logged. The zero value denies all hosts.
type TargetSet struct {
	byProto map[ProtocolID][]Target
	allowed *hostAllow // O(1) host membership, incl. explicit allowed-domains
}

// hostAllow is the O(1) host membership set backing TargetSet.Permits. Hosts
// are stored lowercased with any port stripped, so lookups are a single map hit.
type hostAllow struct {
	hosts map[string]struct{}
}

func (h *hostAllow) permits(host string) bool {
	if h == nil || len(h.hosts) == 0 {
		return false
	}
	_, ok := h.hosts[normalizeHost(host)]
	return ok
}

// normalizeHost lowercases and strips a trailing :port (and a trailing dot)
// so allowlist membership is host-only and case-insensitive.
func normalizeHost(hostport string) string {
	h := strings.TrimSpace(hostport)
	if h == "" {
		return ""
	}
	// Strip :port if present. Bracketed IPv6 (e.g. "[::1]:53") keeps the
	// bracketed address; we cut at the last colon only when it follows ']'
	// or when there is exactly one colon (IPv4/hostname:port).
	if i := strings.LastIndexByte(h, ':'); i >= 0 {
		if strings.HasPrefix(h, "[") {
			if j := strings.IndexByte(h, ']'); j >= 0 && i > j {
				h = h[:i]
			}
		} else if strings.Count(h, ":") == 1 {
			h = h[:i]
		}
	}
	h = strings.TrimSuffix(h, ".")
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")
	return strings.ToLower(h)
}

// NewTargetSet builds the frozen allowlist from the resolved targets plus any
// explicit allowed-domains. Both the target hosts and the allowed-domains are
// admitted to the O(1) membership set. Called once at Scenario.Validate; the
// result is immutable for the run's lifetime (design §5: static target set).
func NewTargetSet(targets []Target, allowedDomains []string) TargetSet {
	byProto := make(map[ProtocolID][]Target)
	hosts := make(map[string]struct{})
	for _, t := range targets {
		byProto[t.Proto] = append(byProto[t.Proto], t)
		if hn := normalizeHost(t.Addr); hn != "" {
			hosts[hn] = struct{}{}
		}
	}
	for _, d := range allowedDomains {
		if hn := normalizeHost(d); hn != "" {
			hosts[hn] = struct{}{}
		}
	}
	return TargetSet{byProto: byProto, allowed: &hostAllow{hosts: hosts}}
}

// Permits reports whether host is in the authoritative allowlist (O(1)). It is
// the single default-deny gate every handler consults before any wire request,
// redirect hop, or discovered-link follow (design §7). The zero-value TargetSet
// denies every host.
func (ts TargetSet) Permits(host string) bool {
	return ts.allowed.permits(host)
}

// TargetsFor returns the targets registered for a protocol (nil if none).
func (ts TargetSet) TargetsFor(p ProtocolID) []Target {
	return ts.byProto[p]
}
