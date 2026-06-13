// Package clock provides an injectable wall-clock abstraction. Every clock read
// and timer in PhantomTraffic goes through this seam so a deterministic fake can
// fast-forward schedules in tests (design §1, §2). Production code uses NewReal;
// tests use NewFake.
package clock

import (
	"context"
	"time"
)

// Timer is the minimal, mockable timer the Clock hands out. The real clock backs
// it with a *time.Timer; the fake backs it with a channel it fires on Advance.
type Timer interface {
	C() <-chan time.Time // fires once at the timer's deadline
	Stop() bool          // reports whether the timer was stopped before firing
}

// Clock is the injected time source. Signatures are authoritative (design §2,
// lines 286-293). Sleep returns ctx.Err() on cancellation. NewTimer/AfterFunc
// drive the ramp/schedule/rotation/session-cap governors.
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	Sleep(ctx context.Context, d time.Duration) error // returns ctx.Err() on cancel
	NewTimer(d time.Duration) Timer
	AfterFunc(d time.Duration, f func()) Timer
}

// realClock is the production Clock; it wraps the stdlib time package directly.
type realClock struct{}

// NewReal returns a Clock backed by the real system clock.
func NewReal() Clock { return realClock{} }

func (realClock) Now() time.Time { return time.Now() }

func (realClock) Since(t time.Time) time.Duration { return time.Since(t) }

// Sleep blocks for d or until ctx is canceled, whichever comes first. It returns
// ctx.Err() on cancellation (design §2). A non-positive d returns immediately.
func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (realClock) NewTimer(d time.Duration) Timer {
	return realTimer{t: time.NewTimer(d)}
}

func (realClock) AfterFunc(d time.Duration, f func()) Timer {
	return realTimer{t: time.AfterFunc(d, f)}
}

// realTimer adapts *time.Timer to the Timer interface.
type realTimer struct{ t *time.Timer }

func (r realTimer) C() <-chan time.Time { return r.t.C }

func (r realTimer) Stop() bool { return r.t.Stop() }

// compile-time assertions.
var (
	_ Clock = realClock{}
	_ Timer = realTimer{}
)
