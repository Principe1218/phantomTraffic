package engine

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/protocols"
)

// StatsSnapshot is an immutable point-in-time view folded from the collector
// shards. It is published copy-on-write and is safe to share across goroutines.
type StatsSnapshot struct {
	At                                 time.Time
	Requests, Successes, Failures      int64
	Reconnects, Panics                 int64
	BytesIn, BytesOut, ActiveConns     int64
	CapSaturationPct                   float64
	LatencyP50, LatencyP90, LatencyP99 time.Duration
	PerTarget                          map[string]TargetStats
}

// TargetStats is the per-target slice of a snapshot.
type TargetStats struct {
	Requests, Successes, Failures int64
	BytesIn, BytesOut             int64
}

// counterShard is the atomic per-target (and global) counter set. One writer per
// worker shard on the hot path; folded read-only on snapshot. Reconnects and
// panics are tracked distinctly so pause artifacts and crashes never pollute the
// success/failure ratio (foundation §8).
type counterShard struct {
	requests, successes, failures atomic.Int64
	reconnects, panics            atomic.Int64
	bytesIn, bytesOut             atomic.Int64
	hist                          histogram // single-writer; read under no lock at snapshot
}

func (s *counterShard) record(r protocols.Result) {
	s.requests.Add(1)
	switch r.Outcome {
	case protocols.OutcomeSuccess:
		s.successes.Add(1)
	case protocols.OutcomeReconnect:
		s.reconnects.Add(1)
	case protocols.OutcomePanicked:
		s.panics.Add(1)
	case protocols.OutcomeFailure:
		s.failures.Add(1)
	// OutcomeSkipped and OutcomeCancelled are benign non-events; not failures.
	}
	if r.BytesIn != 0 {
		s.bytesIn.Add(r.BytesIn)
	}
	if r.BytesOut != 0 {
		s.bytesOut.Add(r.BytesOut)
	}
	s.hist.record(r.Latency)
}

// collector implements protocols.StatsRecorder over a growable target set.
// Shards are built at scenario start; ApplyPatch.TargetsAdd may append under
// patchMu (the engine write barrier). mu guards shards+targetIDs so the
// snapshot reader and the rare addShard writer never race.
type collector struct {
	global     counterShard
	shards     map[string]*counterShard // guarded by mu
	mu         sync.RWMutex
	targetIDs  []string // guarded by mu
	active     atomic.Int64
	clk        clock.Clock
	saturation func() float64
}

func newCounterShard() *counterShard { return &counterShard{} }

func newCollector(targetIDs []string, clk clock.Clock, saturation func() float64) *collector {
	if saturation == nil {
		saturation = func() float64 { return 0 }
	}
	shards := make(map[string]*counterShard, len(targetIDs))
	ids := make([]string, 0, len(targetIDs))
	for _, id := range targetIDs {
		shards[id] = newCounterShard()
		ids = append(ids, id)
	}
	return &collector{shards: shards, targetIDs: ids, clk: clk, saturation: saturation}
}

// Record implements protocols.StatsRecorder. Hot path: one RLock for the map
// lookup, then atomics on the shard. Unknown targets count globally only.
func (c *collector) Record(r protocols.Result) {
	c.global.record(r)
	c.mu.RLock()
	shard := c.shards[r.Target]
	c.mu.RUnlock()
	if shard != nil {
		shard.record(r)
	}
}

// addShard appends a per-target shard under the write barrier. It is safe ONLY
// when called from ApplyPatch under r.patchMu; the RWMutex guards against
// concurrent snapshot reads.
func (c *collector) addShard(targetID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.shards[targetID]; ok {
		return // idempotent
	}
	c.shards[targetID] = newCounterShard()
	c.targetIDs = append(c.targetIDs, targetID)
}

func (c *collector) incActive() { c.active.Add(1) }
func (c *collector) decActive() { c.active.Add(-1) }

// snapshot folds shards read-only into an immutable StatsSnapshot. It takes
// the read lock to capture a consistent view of shards+targetIDs.
func (c *collector) snapshot() StatsSnapshot {
	c.mu.RLock()
	ids := make([]string, len(c.targetIDs))
	copy(ids, c.targetIDs)
	shardsCopy := make(map[string]*counterShard, len(c.shards))
	for k, v := range c.shards {
		shardsCopy[k] = v
	}
	c.mu.RUnlock()

	perTarget := make(map[string]TargetStats, len(ids))
	var merged histogram
	for _, id := range ids {
		s := shardsCopy[id]
		perTarget[id] = TargetStats{
			Requests:  s.requests.Load(),
			Successes: s.successes.Load(),
			Failures:  s.failures.Load(),
			BytesIn:   s.bytesIn.Load(),
			BytesOut:  s.bytesOut.Load(),
		}
	}
	// Percentiles come from the global shard's histogram (every Record folds into
	// global), which is already the union of all per-target latency samples.
	merged.merge(&c.global.hist)
	return StatsSnapshot{
		At:               c.clk.Now(),
		Requests:         c.global.requests.Load(),
		Successes:        c.global.successes.Load(),
		Failures:         c.global.failures.Load(),
		Reconnects:       c.global.reconnects.Load(),
		Panics:           c.global.panics.Load(),
		BytesIn:          c.global.bytesIn.Load(),
		BytesOut:         c.global.bytesOut.Load(),
		ActiveConns:      c.active.Load(),
		CapSaturationPct: c.saturation(),
		LatencyP50:       merged.percentile(0.50),
		LatencyP90:       merged.percentile(0.90),
		LatencyP99:       merged.percentile(0.99),
		PerTarget:        perTarget,
	}
}

// compile-time assertion that collector satisfies the pinned recorder contract.
var _ protocols.StatsRecorder = (*collector)(nil)
