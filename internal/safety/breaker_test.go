package safety

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
)

// A fresh breaker is Closed and admits.
func TestBreakerStartsClosed(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	b := NewBreaker(clk, 3, time.Minute)
	if b.State() != BreakerClosed {
		t.Fatalf("fresh State = %v, want BreakerClosed", b.State())
	}
	if !b.Allow() {
		t.Fatal("fresh breaker should Allow")
	}
}

// N consecutive failures open the breaker; a success before the threshold
// resets the failure count.
func TestBreakerOpensAfterThreshold(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	b := NewBreaker(clk, 3, time.Minute)

	b.RecordFailure()
	b.RecordFailure()
	b.RecordSuccess() // resets the streak
	if b.State() != BreakerClosed {
		t.Fatalf("after success State = %v, want BreakerClosed", b.State())
	}

	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure() // third consecutive => open
	if b.State() != BreakerOpen {
		t.Fatalf("after threshold State = %v, want BreakerOpen", b.State())
	}
	if b.Allow() {
		t.Fatal("open breaker should deny before cooldown")
	}
}

// After the cooldown elapses, Allow returns true exactly once and moves the
// breaker to half-open; a probe success closes it, a probe failure re-opens it.
func TestBreakerHalfOpenProbe(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	cooldown := time.Minute
	b := NewBreaker(clk, 1, cooldown)

	b.RecordFailure() // threshold 1 => open immediately
	if b.State() != BreakerOpen {
		t.Fatalf("State = %v, want BreakerOpen", b.State())
	}

	// Before cooldown: denied, still open.
	clk.Advance(cooldown - time.Second)
	if b.Allow() {
		t.Fatal("before cooldown Allow should be false")
	}

	// After cooldown: one probe allowed, breaker is half-open.
	clk.Advance(2 * time.Second)
	if !b.Allow() {
		t.Fatal("after cooldown the first Allow should permit a probe")
	}
	if b.State() != BreakerHalfOpen {
		t.Fatalf("State after probe-allow = %v, want BreakerHalfOpen", b.State())
	}
	// A second Allow while half-open (probe in flight) is denied.
	if b.Allow() {
		t.Fatal("half-open should allow only a single probe at a time")
	}

	// Probe success closes the breaker.
	b.RecordSuccess()
	if b.State() != BreakerClosed {
		t.Fatalf("after probe success State = %v, want BreakerClosed", b.State())
	}
}

// A half-open probe failure re-opens the breaker and restarts the cooldown.
func TestBreakerHalfOpenFailureReopens(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	cooldown := time.Minute
	b := NewBreaker(clk, 1, cooldown)

	b.RecordFailure()
	clk.Advance(cooldown + time.Second)
	if !b.Allow() {
		t.Fatal("probe should be allowed after cooldown")
	}
	b.RecordFailure() // probe failed
	if b.State() != BreakerOpen {
		t.Fatalf("after probe failure State = %v, want BreakerOpen", b.State())
	}
	// Cooldown restarted: still denied right away.
	if b.Allow() {
		t.Fatal("re-opened breaker should deny before the new cooldown")
	}
}

// ForceOpen (TargetsDisable) opens the breaker regardless of failure history
// and stays open through the cooldown only via the half-open probe path.
func TestBreakerForceOpen(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	b := NewBreaker(clk, 3, time.Minute)
	b.ForceOpen()
	if b.State() != BreakerOpen {
		t.Fatalf("State after ForceOpen = %v, want BreakerOpen", b.State())
	}
	if b.Allow() {
		t.Fatal("force-opened breaker should deny before cooldown")
	}
}
