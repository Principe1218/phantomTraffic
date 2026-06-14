package safety

import (
	"context"
	"sync"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// Limiter enforces a two-tier (per-target + global) request budget plus an
// optional streaming byte budget, all over the injected clock. A hard cap
// breach or a latched tripwire surfaces a *pterr.Error{Class: ClassSafety}.
type Limiter interface {
	// Acquire reserves one request slot against both the per-target and global
	// buckets (and estBytes against the streaming byte budget when configured).
	// Returns ClassSafety on a hard cap breach or when the tripwire is tripped;
	// respects ctx cancellation.
	Acquire(ctx context.Context, targetID string, estBytes int64) (Reservation, error)
	// Saturation reports the fraction of wall time blocked in Acquire.
	Saturation() float64
}

// Reservation is the handle returned by a successful Acquire.
type Reservation interface {
	Reconcile(actualBytes int64) // replace the byte estimate with observed bytes
	Release()                    // idempotent; always deferred
}

type limiter struct {
	clk clock.Clock
	tw  *Tripwire

	mu        sync.Mutex
	perTarget map[string]*tokenBucket
	global    *tokenBucket
	bytes     *tokenBucket // nil when StreamingByteRateKbps <= 0
	ptRPS     float64
	gRPS      float64
	byteRate  float64 // bytes/sec; 0 => no byte budget

	blockedNanos int64     // accumulated time blocked in Acquire
	sinceStart   time.Time // limiter creation instant (clock time)
}

// NewLimiter builds a two-tier limiter. estBytes are reserved against
// StreamingByteRateKbps when it is > 0; otherwise the byte tier is inert.
func NewLimiter(clk clock.Clock, caps CapSpec, tw *Tripwire) Limiter {
	byteRate := float64(caps.StreamingByteRateKbps) * 1000.0 / 8.0 // Kbps -> bytes/sec
	l := &limiter{
		clk:        clk,
		tw:         tw,
		perTarget:  make(map[string]*tokenBucket),
		global:     newTokenBucket(clk, caps.GlobalRPS, max1(caps.GlobalRPS)),
		ptRPS:      caps.PerTargetRPS,
		gRPS:       caps.GlobalRPS,
		byteRate:   byteRate,
		sinceStart: clk.Now(),
	}
	if byteRate > 0 {
		l.bytes = newTokenBucket(clk, byteRate, byteRate)
	}
	return l
}

// max1 returns a burst of at least 1 token for any positive rate (so a small
// fractional rate can still admit a single request once accrued).
func max1(rps float64) float64 {
	if rps <= 0 {
		return 0
	}
	if rps < 1 {
		return 1
	}
	return rps
}

func (l *limiter) bucketFor(targetID string) *tokenBucket {
	b, ok := l.perTarget[targetID]
	if !ok {
		b = newTokenBucket(l.clk, l.ptRPS, max1(l.ptRPS))
		l.perTarget[targetID] = b
	}
	return b
}

func (l *limiter) Acquire(ctx context.Context, targetID string, estBytes int64) (Reservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if l.tw != nil && l.tw.Tripped() {
		return nil, safetyErr("tripwire latched: " + l.tw.Reason())
	}

	start := l.clk.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	pt := l.bucketFor(targetID)
	if !pt.tryTake(1) {
		l.recordBlocked(start)
		return nil, safetyErr("per-target request cap exceeded for " + targetID)
	}
	if !l.global.tryTake(1) {
		pt.tokens++ // return the per-target token we just took
		l.recordBlocked(start)
		return nil, safetyErr("global request cap exceeded")
	}
	reserved := float64(0)
	if l.bytes != nil && estBytes > 0 {
		reserved = float64(estBytes)
		if !l.bytes.tryTake(reserved) {
			pt.tokens++
			l.global.tokens++
			l.recordBlocked(start)
			return nil, safetyErr("streaming byte cap exceeded")
		}
	}
	return &reservation{lim: l, reservedBytes: reserved}, nil
}

// recordBlocked accumulates the wall (clock) time spent in a blocked Acquire.
func (l *limiter) recordBlocked(start time.Time) {
	l.blockedNanos += int64(l.clk.Since(start))
}

func (l *limiter) Saturation() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	total := l.clk.Since(l.sinceStart)
	if total <= 0 {
		return 0
	}
	sat := float64(l.blockedNanos) / float64(total)
	if sat > 1 {
		sat = 1
	}
	return sat
}

// returnBytes credits unused byte budget back to the byte bucket, capped at
// burst. Called under l.mu.
func (l *limiter) returnBytes(n float64) {
	if l.bytes == nil || n <= 0 {
		return
	}
	l.bytes.tokens += n
	if l.bytes.tokens > l.bytes.burst {
		l.bytes.tokens = l.bytes.burst
	}
}

type reservation struct {
	lim           *limiter
	mu            sync.Mutex
	reservedBytes float64
	actualBytes   float64
	reconciled    bool
	released      bool
}

func (r *reservation) Reconcile(actualBytes int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actualBytes = float64(actualBytes)
	r.reconciled = true
}

func (r *reservation) Release() {
	r.mu.Lock()
	if r.released {
		r.mu.Unlock()
		return
	}
	r.released = true
	// Return any byte budget we reserved but did not actually consume.
	refund := r.reservedBytes
	if r.reconciled && r.actualBytes < r.reservedBytes {
		refund = r.reservedBytes - r.actualBytes
	} else if r.reconciled {
		refund = 0
	}
	r.mu.Unlock()

	r.lim.mu.Lock()
	r.lim.returnBytes(refund)
	r.lim.mu.Unlock()
}

func safetyErr(msg string) *pterr.Error {
	return pterr.New(pterr.ClassSafety, "safety.cap", "limiter.Acquire", msg)
}
