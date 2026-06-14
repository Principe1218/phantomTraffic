package engine

import (
	"sync"
	"sync/atomic"
)

// subBuffer is the per-subscriber channel depth. A subscriber that falls behind
// has stale values dropped (latest-wins); the buffer only smooths a brief lag.
const subBuffer = 1

// publisher fans a StatsSnapshot out to subscribers without ever backpressuring
// the producer. latest() is lock-free via an atomic pointer; the subscriber set
// is mutex-guarded (mutated only on subscribe/unsubscribe, off the publish hot
// path's lock except for a short read).
type publisher struct {
	cur  atomic.Pointer[StatsSnapshot]
	mu   sync.Mutex
	subs map[chan StatsSnapshot]struct{}
}

func newPublisher(initial StatsSnapshot) *publisher {
	p := &publisher{subs: make(map[chan StatsSnapshot]struct{})}
	s := initial
	p.cur.Store(&s)
	return p
}

// publish swaps the latest pointer and does a non-blocking send to each
// subscriber. A full buffer means the subscriber is slow: drop the stale value
// it has not yet read and overwrite with the newest, so the subscriber always
// converges on the latest snapshot and the producer never blocks.
func (p *publisher) publish(s StatsSnapshot) {
	snap := s
	p.cur.Store(&snap)

	p.mu.Lock()
	defer p.mu.Unlock()
	for ch := range p.subs {
		for {
			select {
			case ch <- snap:
				// delivered (or buffered)
			default:
				// buffer full: drop one stale value, then retry once.
				select {
				case <-ch:
					continue
				default:
				}
			}
			break
		}
	}
}

// latest returns the most recently published snapshot, lock-free.
func (p *publisher) latest() StatsSnapshot {
	return *p.cur.Load()
}

// subscribe returns the current latest, a buffered receive channel, and an
// unsubscribe func that removes and closes the channel (idempotent).
func (p *publisher) subscribe() (StatsSnapshot, <-chan StatsSnapshot, func()) {
	ch := make(chan StatsSnapshot, subBuffer)

	p.mu.Lock()
	p.subs[ch] = struct{}{}
	p.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			p.mu.Lock()
			delete(p.subs, ch)
			p.mu.Unlock()
			close(ch)
		})
	}
	return p.latest(), ch, cancel
}
