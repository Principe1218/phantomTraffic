package engine

import (
	"context"
	"sync"
)

// semaphore is a resizable counting semaphore. Unlike a buffered channel its
// capacity (limit) can change at runtime: the ramp governor raises it to admit
// more vusers and lowers it to drain. Raising the limit wakes blocked waiters;
// lowering it merely stops issuing new permits (in-flight holders keep their slot
// and release naturally — drain, never kill; design §6.6).
type semaphore struct {
	mu    sync.Mutex
	cond  *sync.Cond
	limit int
	held  int
}

// newSemaphore returns a semaphore admitting up to limit concurrent holders.
func newSemaphore(limit int) *semaphore {
	s := &semaphore{limit: limit}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// acquire blocks until a permit is available (held < limit) or ctx is done, in
// which case it returns ctx.Err() and consumes no permit. It is ctx-first so STOP
// (runCtx cancel) unblocks every waiter (design §6.3).
func (s *semaphore) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Watcher goroutine broadcasts on cancel so a Cond.Wait blocked here wakes.
	// It exits as soon as acquire returns (done closed), so it never leaks.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			s.cond.Broadcast()
		case <-done:
		}
	}()

	s.mu.Lock()
	defer s.mu.Unlock()
	for s.held >= s.limit {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.cond.Wait()
	}
	s.held++
	return nil
}

// release returns one permit and wakes a single waiter.
func (s *semaphore) release() {
	s.mu.Lock()
	if s.held > 0 {
		s.held--
	}
	s.mu.Unlock()
	s.cond.Signal()
}

// currentLimit returns the semaphore's current admission limit.
func (s *semaphore) currentLimit() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.limit
}

// setLimit resizes the live concurrency budget. Raising it wakes waiters that can
// now proceed; lowering it just stops issuing (held may temporarily exceed limit).
func (s *semaphore) setLimit(n int) {
	s.mu.Lock()
	s.limit = n
	s.mu.Unlock()
	s.cond.Broadcast()
}
