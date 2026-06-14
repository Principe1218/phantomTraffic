package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestSemaphoreAcquireRelease verifies the basic count: acquire up to the limit
// succeeds without blocking, and release frees a slot for a later acquire.
func TestSemaphoreAcquireRelease(t *testing.T) {
	s := newSemaphore(2)
	ctx := context.Background()

	if err := s.acquire(ctx); err != nil {
		t.Fatalf("first acquire: unexpected error %v", err)
	}
	if err := s.acquire(ctx); err != nil {
		t.Fatalf("second acquire: unexpected error %v", err)
	}

	// A third acquire must block; run it in a goroutine and confirm it does not
	// complete until a slot is released.
	done := make(chan error, 1)
	go func() { done <- s.acquire(ctx) }()

	select {
	case err := <-done:
		t.Fatalf("third acquire returned %v while at limit; expected it to block", err)
	case <-time.After(20 * time.Millisecond):
		// still blocked as expected
	}

	s.release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("third acquire after release: unexpected error %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("third acquire did not unblock after release")
	}
}

// TestSemaphoreSetLimitGrowWakesWaiters verifies that raising the limit wakes a
// blocked waiter (the ramp governor growing concurrency mid-run).
func TestSemaphoreSetLimitGrowWakesWaiters(t *testing.T) {
	s := newSemaphore(1)
	ctx := context.Background()

	if err := s.acquire(ctx); err != nil {
		t.Fatalf("acquire: unexpected error %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- s.acquire(ctx) }()

	// Let the second acquire register as a waiter.
	time.Sleep(20 * time.Millisecond)

	// Growing the limit must wake the waiter even though nothing was released.
	s.setLimit(2)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waiter after grow: unexpected error %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("raising the limit did not wake the blocked waiter")
	}
}

// TestSemaphoreLowerLimitStopsIssuing verifies that lowering the limit below the
// held count does not panic and simply stops issuing new permits: a fresh acquire
// blocks until enough releases bring held back under the (lower) limit.
func TestSemaphoreLowerLimitStopsIssuing(t *testing.T) {
	s := newSemaphore(3)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := s.acquire(ctx); err != nil {
			t.Fatalf("acquire %d: unexpected error %v", i, err)
		}
	}

	// Lower the limit while 3 are held. No panic; new acquires block.
	s.setLimit(1)

	done := make(chan error, 1)
	go func() { done <- s.acquire(ctx) }()
	select {
	case err := <-done:
		t.Fatalf("acquire returned %v while held(3) >= limit(1); expected block", err)
	case <-time.After(20 * time.Millisecond):
	}

	// Releasing twice brings held to 1, still not under limit(1); one more release
	// brings held to 0 < limit(1) so the waiter proceeds.
	s.release()
	s.release()
	select {
	case err := <-done:
		t.Fatalf("acquire returned %v while held(1) >= limit(1); expected block", err)
	case <-time.After(20 * time.Millisecond):
	}
	s.release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waiter: unexpected error %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter did not proceed after held dropped below limit")
	}
}

// TestSemaphoreAcquireCtxCancel verifies a blocked acquire returns ctx.Err() when
// the context is canceled (STOP propagation).
func TestSemaphoreAcquireCtxCancel(t *testing.T) {
	s := newSemaphore(1)
	if err := s.acquire(context.Background()); err != nil {
		t.Fatalf("acquire: unexpected error %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.acquire(ctx) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("acquire after cancel: got %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("acquire did not return after ctx cancel")
	}
}
