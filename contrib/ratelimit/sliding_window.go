package ratelimit

import "time"

// slidingWindowAllow implements the sliding window counter algorithm.
// Uses weighted average of current and previous window counts for smooth rate limiting.
// Zero heap allocation on the hot path.
func slidingWindowAllow(e *entry, limit int, window time.Duration) Result {
	now := time.Now()
	e.mu.Lock()

	// Initialize on first request
	if e.windowStart.IsZero() {
		e.windowStart = now
		e.count = 1
		e.prevCount = 0
		e.last = now
		e.mu.Unlock()
		return Result{
			Allowed:   true,
			Remaining: limit - 1,
			ResetAt:   now.Add(window),
		}
	}

	// Advance windows if needed
	elapsed := now.Sub(e.windowStart)
	if elapsed >= 2*window {
		// Skipped an entire window — reset both
		e.prevCount = 0
		e.count = 0
		e.windowStart = now
	} else if elapsed >= window {
		// Current window becomes previous, start new window
		e.prevCount = e.count
		e.count = 0
		e.windowStart = e.windowStart.Add(window)
	}

	// Calculate weighted count using sliding window position
	windowElapsed := now.Sub(e.windowStart)
	weight := 1.0 - (windowElapsed.Seconds() / window.Seconds())
	if weight < 0 {
		weight = 0
	}
	estimatedCount := float64(e.prevCount)*weight + float64(e.count)

	resetAt := e.windowStart.Add(window)

	if estimatedCount >= float64(limit) {
		// Rejected
		retryAfter := resetAt.Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		remaining := int(float64(limit) - estimatedCount)
		if remaining < 0 {
			remaining = 0
		}
		e.mu.Unlock()
		return Result{
			Allowed:   false,
			Remaining: remaining,
			ResetAt:   resetAt,
			RetryAt:   retryAfter,
		}
	}

	// Allowed — increment count
	e.count++
	e.last = now
	remaining := int(float64(limit) - (float64(e.prevCount)*weight + float64(e.count)))
	if remaining < 0 {
		remaining = 0
	}
	e.mu.Unlock()

	return Result{
		Allowed:   true,
		Remaining: remaining,
		ResetAt:   resetAt,
	}
}
