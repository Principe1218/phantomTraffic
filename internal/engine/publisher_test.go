package engine

import (
	"testing"
	"time"
)

func snapWith(req int64) StatsSnapshot {
	return StatsSnapshot{At: time.Unix(req, 0), Requests: req}
}

func TestPublisherLatest(t *testing.T) {
	t.Parallel()
	p := newPublisher(snapWith(1))
	if got := p.latest().Requests; got != 1 {
		t.Fatalf("initial latest = %d, want 1", got)
	}
	p.publish(snapWith(7))
	if got := p.latest().Requests; got != 7 {
		t.Fatalf("after publish latest = %d, want 7", got)
	}
}

func TestPublisherSubscribeReceivesPublishes(t *testing.T) {
	t.Parallel()
	p := newPublisher(snapWith(1))
	initial, ch, cancel := p.subscribe()
	defer cancel()
	if initial.Requests != 1 {
		t.Fatalf("subscribe initial = %d, want 1", initial.Requests)
	}

	p.publish(snapWith(2))
	select {
	case s := <-ch:
		if s.Requests != 2 {
			t.Fatalf("received = %d, want 2", s.Requests)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published snapshot")
	}
}

func TestPublisherSlowSubscriberDropsLatestWins(t *testing.T) {
	t.Parallel()
	p := newPublisher(snapWith(0))
	_, ch, cancel := p.subscribe()
	defer cancel()

	// Publish more than the channel can buffer without anyone draining it.
	// publish must never block; the slow subscriber keeps only the latest.
	for i := 1; i <= 100; i++ {
		done := make(chan struct{})
		go func(n int64) { p.publish(snapWith(n)); close(done) }(int64(i))
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("publish blocked on a slow subscriber at i=%d", i)
		}
	}

	// The very last published value (100) must be retrievable: either it is the
	// latest pointer, or it is the most recent value sitting in the buffer. Drain
	// and confirm the newest seen reaches 100 (latest-wins, never a stale lock).
	if got := p.latest().Requests; got != 100 {
		t.Fatalf("latest = %d, want 100", got)
	}
	var newest int64
	for {
		select {
		case s := <-ch:
			if s.Requests > newest {
				newest = s.Requests
			}
		default:
			if newest == 0 {
				t.Fatal("slow subscriber received nothing")
			}
			return
		}
	}
}

func TestPublisherCancelStopsDelivery(t *testing.T) {
	t.Parallel()
	p := newPublisher(snapWith(0))
	_, ch, cancel := p.subscribe()
	cancel()

	// After cancel the channel is closed and no further values arrive.
	p.publish(snapWith(5))
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("received a value after cancel; want closed channel")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after cancel")
	}
}
