package engine

import (
	"sync"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
	"github.com/Principe1218/phantomTraffic/internal/rng"
	"github.com/Principe1218/phantomTraffic/internal/scenario"
)

// protoState is the per-protocol rotation cursor: the current index into the
// protocol's target slice and the clock time of the last rotation.
type protoState struct {
	targets    []protocols.Target
	idx        int
	lastRotate time.Time
}

// rotatingSelector is a clock-aware, internally-synchronized TargetSelector shared
// across all vusers of one scenario block. Rotation is interval-driven so the whole
// block moves together ("Target 1 for 5 minutes, then everyone rotates"). It
// implements behavior.TargetSelector. interval <= 0 means never rotate (stay on
// index 0). The mutex guards the per-protocol cursors; it is off the per-request hot
// path because rotation state changes only when an interval elapses.
type rotatingSelector struct {
	mu       sync.Mutex
	clk      clock.Clock
	rand     rng.Rand
	strategy scenario.RotationStrategy
	interval time.Duration
	byProto  map[protocols.ProtocolID]*protoState
}

// newRotatingSelector builds a selector over the block's per-protocol target lists.
// lastRotate is seeded to the clock's current time so the first interval is measured
// from construction.
func newRotatingSelector(
	clk clock.Clock,
	r rng.Rand,
	strategy scenario.RotationStrategy,
	interval time.Duration,
	byProto map[protocols.ProtocolID][]protocols.Target,
) *rotatingSelector {
	now := clk.Now()
	states := make(map[protocols.ProtocolID]*protoState, len(byProto))
	for proto, targets := range byProto {
		states[proto] = &protoState{targets: targets, idx: 0, lastRotate: now}
	}
	return &rotatingSelector{
		clk:      clk,
		rand:     r,
		strategy: strategy,
		interval: interval,
		byProto:  states,
	}
}

// Next returns the current target for proto, advancing the rotation cursor first if
// the interval has elapsed. ok is false (a benign skip) when the protocol has no
// targets configured.
func (s *rotatingSelector) Next(proto protocols.ProtocolID) (protocols.Target, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.byProto[proto]
	if !ok || len(st.targets) == 0 {
		return protocols.Target{}, false
	}

	s.maybeRotate(st)
	return st.targets[st.idx], true
}

// maybeRotate advances st.idx when interval > 0 and at least one full interval has
// elapsed since the last rotation. Caller holds s.mu.
func (s *rotatingSelector) maybeRotate(st *protoState) {
	if s.interval <= 0 {
		return
	}
	if s.clk.Since(st.lastRotate) < s.interval {
		return
	}
	n := len(st.targets)
	switch s.strategy {
	case scenario.RotationRandom:
		st.idx = s.rand.IntN(n)
	default: // scenario.RotationSequential
		st.idx = (st.idx + 1) % n
	}
	st.lastRotate = s.clk.Now()
}
