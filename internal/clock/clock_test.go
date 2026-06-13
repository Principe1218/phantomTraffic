package clock

import (
	"context"
	"testing"
	"time"
)

// compile-time assertion that the real clock satisfies the interface.
var _ Clock = NewReal()

func TestRealNowAndSince(t *testing.T) {
	c := NewReal()
	start := c.Now()
	time.Sleep(5 * time.Millisecond)
	if el := c.Since(start); el < 4*time.Millisecond {
		t.Fatalf("Since too small: %v", el)
	}
}

func TestRealSleepReturnsAfterDuration(t *testing.T) {
	c := NewReal()
	start := time.Now()
	if err := c.Sleep(context.Background(), 20*time.Millisecond); err != nil {
		t.Fatalf("Sleep returned error: %v", err)
	}
	if el := time.Since(start); el < 15*time.Millisecond {
		t.Fatalf("Sleep returned too soon: %v", el)
	}
}

func TestRealSleepCancelReturnsCtxErr(t *testing.T) {
	c := NewReal()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	if err := c.Sleep(ctx, time.Hour); err != context.Canceled {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRealSleepAlreadyCancelled(t *testing.T) {
	c := NewReal()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Sleep(ctx, time.Hour); err != context.Canceled {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRealTimerFires(t *testing.T) {
	c := NewReal()
	tm := c.NewTimer(10 * time.Millisecond)
	select {
	case <-tm.C():
	case <-time.After(time.Second):
		t.Fatal("real timer did not fire")
	}
}

func TestRealTimerStopPreventsFire(t *testing.T) {
	c := NewReal()
	tm := c.NewTimer(time.Hour)
	if !tm.Stop() {
		t.Fatal("Stop on a long pending timer should return true")
	}
}

func TestRealAfterFunc(t *testing.T) {
	c := NewReal()
	done := make(chan struct{})
	c.AfterFunc(10*time.Millisecond, func() { close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("AfterFunc callback never ran")
	}
}

// ---- fake clock ----

var fakeBase = time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

func TestFakeSatisfiesClock(t *testing.T) {
	var _ Clock = NewFake(fakeBase)
}

func TestFakeNowAndSince(t *testing.T) {
	c := NewFake(fakeBase)
	if !c.Now().Equal(fakeBase) {
		t.Fatalf("Now=%v want %v", c.Now(), fakeBase)
	}
	c.Advance(10 * time.Second)
	if !c.Now().Equal(fakeBase.Add(10 * time.Second)) {
		t.Fatalf("Now after advance=%v", c.Now())
	}
	if got := c.Since(fakeBase); got != 10*time.Second {
		t.Fatalf("Since=%v want 10s", got)
	}
}

func TestFakeAdvanceFiresTimer(t *testing.T) {
	c := NewFake(fakeBase)
	tm := c.NewTimer(5 * time.Second)
	select {
	case <-tm.C():
		t.Fatal("timer fired before Advance")
	default:
	}
	c.Advance(5 * time.Second)
	select {
	case got := <-tm.C():
		if !got.Equal(fakeBase.Add(5 * time.Second)) {
			t.Fatalf("timer fired with %v want %v", got, fakeBase.Add(5*time.Second))
		}
	default:
		t.Fatal("timer did not fire after Advance past deadline")
	}
}

func TestFakeAdvancePartialDoesNotFire(t *testing.T) {
	c := NewFake(fakeBase)
	tm := c.NewTimer(5 * time.Second)
	c.Advance(4 * time.Second)
	select {
	case <-tm.C():
		t.Fatal("timer fired before its deadline")
	default:
	}
	c.Advance(1 * time.Second)
	select {
	case <-tm.C():
	default:
		t.Fatal("timer did not fire once deadline reached")
	}
}

func TestFakeAfterFuncRunsOnAdvance(t *testing.T) {
	c := NewFake(fakeBase)
	done := make(chan struct{})
	c.AfterFunc(2*time.Second, func() { close(done) })
	c.Advance(2 * time.Second)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("AfterFunc callback not invoked on Advance")
	}
}

func TestFakeStopPreventsFire(t *testing.T) {
	c := NewFake(fakeBase)
	tm := c.NewTimer(5 * time.Second)
	if !tm.Stop() {
		t.Fatal("Stop on a pending timer should return true")
	}
	c.Advance(10 * time.Second)
	select {
	case <-tm.C():
		t.Fatal("stopped timer fired")
	default:
	}
	if tm.Stop() {
		t.Fatal("second Stop should return false")
	}
}

func TestFakeSleepUnblocksOnAdvance(t *testing.T) {
	c := NewFake(fakeBase)
	done := make(chan error, 1)
	go func() { done <- c.Sleep(context.Background(), 3*time.Second) }()
	// give the goroutine time to register its waiter before advancing.
	time.Sleep(20 * time.Millisecond)
	c.Advance(3 * time.Second)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Sleep returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Sleep never returned after Advance past deadline")
	}
}

func TestFakeSleepDoesNotUnblockBeforeDeadline(t *testing.T) {
	c := NewFake(fakeBase)
	done := make(chan error, 1)
	go func() { done <- c.Sleep(context.Background(), 3*time.Second) }()
	time.Sleep(20 * time.Millisecond)
	c.Advance(2 * time.Second)
	select {
	case <-done:
		t.Fatal("Sleep returned before deadline reached")
	case <-time.After(50 * time.Millisecond):
	}
	c.Advance(1 * time.Second)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Sleep returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Sleep did not return once deadline reached")
	}
}

func TestFakeSleepCancelReturnsCtxErr(t *testing.T) {
	c := NewFake(fakeBase)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Sleep(ctx, time.Hour) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Sleep did not unblock on cancel")
	}
}

func TestFakeSleepAlreadyCancelled(t *testing.T) {
	c := NewFake(fakeBase)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Sleep(ctx, time.Hour); err != context.Canceled {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestFakeSleepZeroDurationReturnsImmediately(t *testing.T) {
	c := NewFake(fakeBase)
	if err := c.Sleep(context.Background(), 0); err != nil {
		t.Fatalf("zero Sleep returned error: %v", err)
	}
}

func TestFakeSetAdvancesAndFires(t *testing.T) {
	c := NewFake(fakeBase)
	tm := c.NewTimer(time.Minute)
	c.Set(fakeBase.Add(time.Minute))
	if !c.Now().Equal(fakeBase.Add(time.Minute)) {
		t.Fatalf("Set did not move Now: %v", c.Now())
	}
	select {
	case <-tm.C():
	default:
		t.Fatal("Set past the deadline did not fire the timer")
	}
}

func TestFakeFiresInDeadlineOrder(t *testing.T) {
	c := NewFake(fakeBase)
	var order []int
	record := func(n int) func() {
		return func() { order = append(order, n) }
	}
	c.AfterFunc(3*time.Second, record(3))
	c.AfterFunc(1*time.Second, record(1))
	c.AfterFunc(2*time.Second, record(2))
	// Callbacks are synchronous in deadline order; order is populated before
	// Advance returns — no sleep or mutex needed.
	c.Advance(5 * time.Second)
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("callbacks fired out of deadline order: %v", order)
	}
}
