package behavior

import "github.com/Principe1218/phantomTraffic/internal/protocols"

// TargetSelector returns the next allowlist target for a protocol. The behavior
// layer guarantees allowlist-safety by construction (it only ever returns a
// target the selector was built from). The rotation policy (sequential/random +
// interval, from the scenario) is dispatcher-owned (design §5); Plan 4 injects a
// policy-aware implementation. Plan 3 ships the deterministic round-robin default.
type TargetSelector interface {
	// Next returns the next target for proto and ok=true, or ok=false when the
	// protocol has no configured targets (a benign skip, not an error).
	Next(proto protocols.ProtocolID) (protocols.Target, bool)
}

// RoundRobinSelector cycles deterministically through each protocol's targets.
// It is the Plan-3 default; Plan 4's dispatcher replaces it with a rotation-
// policy-aware selector built from the frozen, allowlist-validated block targets.
type RoundRobinSelector struct {
	byProto map[protocols.ProtocolID][]protocols.Target
	idx     map[protocols.ProtocolID]int
}

// NewRoundRobinSelector builds a selector over per-protocol targets. The caller
// supplies targets already validated against the scenario allowlist.
func NewRoundRobinSelector(byProto map[protocols.ProtocolID][]protocols.Target) *RoundRobinSelector {
	return &RoundRobinSelector{byProto: byProto, idx: make(map[protocols.ProtocolID]int)}
}

func (s *RoundRobinSelector) Next(proto protocols.ProtocolID) (protocols.Target, bool) {
	ts := s.byProto[proto]
	if len(ts) == 0 {
		return protocols.Target{}, false
	}
	i := s.idx[proto] % len(ts)
	s.idx[proto]++
	return ts[i], true
}
