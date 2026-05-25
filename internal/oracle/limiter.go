package oracle

import (
	"context"
	"sync"
	"time"
)

// scryfallLimiter is a leaky-bucket rate limiter that spaces out the
// START of Scryfall API requests by a fixed interval but does NOT
// serialize the requests end-to-end. Multiple goroutines can hold the
// HTTP round-trip in flight at the same time; the limiter only ensures
// no two requests are dispatched within `interval` of each other.
//
// This replaces the previous "mutex + sleep" gate which forced
// serial dispatch AND serial completion — i.e. wall time per call was
// (interval + RTT). With a leaky bucket, wall time for N parallel calls
// is roughly (N * interval) + max(RTT), so a 99-card fuzzy fallback
// against ~250ms Scryfall RTT drops from ~37s to ~13s.
//
// Scryfall's documented ceiling is ~10 req/s with 50-100ms recommended
// between requests. We default to 120ms (about 8.3 req/s) which leaves
// headroom; the interval is tunable for tests via newScryfallLimiter.
type scryfallLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	// next is the earliest wall-clock moment a new request may START.
	// Each successful Wait() reserves its slot by advancing `next` by
	// one interval, so concurrent callers spread out cleanly.
	next time.Time
}

func newScryfallLimiter(interval time.Duration) *scryfallLimiter {
	return &scryfallLimiter{interval: interval}
}

// Wait blocks until the caller's slot opens, then returns. The slot is
// reserved at the moment Wait is called, not when it returns — so 8
// goroutines that call Wait simultaneously each get a unique reservation
// at t, t+interval, t+2·interval, ... and return at those times.
func (l *scryfallLimiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	var sleep time.Duration
	if now.Before(l.next) {
		sleep = l.next.Sub(now)
		l.next = l.next.Add(l.interval)
	} else {
		l.next = now.Add(l.interval)
	}
	l.mu.Unlock()

	if sleep <= 0 {
		return nil
	}
	t := time.NewTimer(sleep)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// setIntervalForTest is a test-only helper that swaps the interval and
// resets the bucket so the next Wait returns immediately. Returns a
// cleanup that restores the prior interval.
func (l *scryfallLimiter) setIntervalForTest(d time.Duration) func() {
	l.mu.Lock()
	orig := l.interval
	l.interval = d
	l.next = time.Time{}
	l.mu.Unlock()
	return func() {
		l.mu.Lock()
		l.interval = orig
		l.next = time.Time{}
		l.mu.Unlock()
	}
}
