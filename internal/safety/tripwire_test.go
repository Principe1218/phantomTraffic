package safety

import (
	"sync"
	"testing"
)

// A budget of 0 (or negative) means unlimited: CountRequest never trips.
func TestTripwireUnlimitedBudget(t *testing.T) {
	tw := NewTripwire(0)
	for i := 0; i < 10_000; i++ {
		if tw.CountRequest() {
			t.Fatalf("unlimited tripwire tripped at request %d", i)
		}
	}
	if tw.Tripped() {
		t.Fatal("unlimited tripwire should never be tripped")
	}
}

// CountRequest trips once the count exceeds the budget, and stays latched.
func TestTripwireBudgetExhaustion(t *testing.T) {
	tw := NewTripwire(3)
	for i := 1; i <= 3; i++ {
		if tw.CountRequest() {
			t.Fatalf("tripped early at request %d (budget 3)", i)
		}
	}
	// Fourth request exceeds budget => trips.
	if !tw.CountRequest() {
		t.Fatal("request beyond budget should trip")
	}
	if !tw.Tripped() {
		t.Fatal("Tripped should be true after budget exhaustion")
	}
	// Latched: further calls stay tripped (no auto-reset).
	if !tw.CountRequest() {
		t.Fatal("tripwire must stay latched")
	}
	if tw.Reason() == "" {
		t.Fatal("budget-exhaustion trip should record a reason")
	}
}

// Trip latches with an explicit reason regardless of budget.
func TestTripwireExplicitTrip(t *testing.T) {
	tw := NewTripwire(0)
	tw.Trip("panic-storm")
	if !tw.Tripped() {
		t.Fatal("explicit Trip should latch")
	}
	if got := tw.Reason(); got != "panic-storm" {
		t.Fatalf("Reason = %q, want %q", got, "panic-storm")
	}
	// First reason wins; a later Trip does not overwrite the latched reason.
	tw.Trip("budget")
	if got := tw.Reason(); got != "panic-storm" {
		t.Fatalf("Reason after re-trip = %q, want %q (no overwrite)", got, "panic-storm")
	}
}

// CountRequest and Trip are safe under concurrency (atomics + mutex on reason).
func TestTripwireConcurrent(t *testing.T) {
	tw := NewTripwire(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				tw.CountRequest()
			}
		}()
	}
	wg.Wait()
	// 500 requests against a budget of 100 => must be tripped.
	if !tw.Tripped() {
		t.Fatal("500 requests over a budget of 100 should trip")
	}
}
