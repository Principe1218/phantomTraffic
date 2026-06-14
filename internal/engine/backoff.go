package engine

import (
	"time"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// backoffDelay computes one retry delay using exponential backoff with full
// jitter (design §7.1). attempt is 0-based:
//
//	exp   = min(max, base << attempt)   // capped exponential
//	delay = Duration(r.Float64() * exp) // full jitter in [0, exp)
//
// Full jitter (rather than equal jitter) maximizes de-correlation of concurrent
// retriers so backoff never synchronizes into a thundering herd. The randomness
// comes from the injected rng.Rand — never math/rand (AGENTS.md §2.2).
func backoffDelay(attempt int, base, max time.Duration, r rng.Rand) time.Duration {
	exp := base << attempt
	if exp <= 0 || exp > max {
		exp = max
	}
	return time.Duration(r.Float64() * float64(exp))
}
