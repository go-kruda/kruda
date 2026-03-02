package ratelimit

import "time"

// tokenBucketAllow implements the token bucket algorithm.
// Tokens refill at a constant rate (limit / window). Each request consumes 1 token.
// Zero heap allocation on the hot path — all state is in the pre-allocated entry.
func tokenBucketAllow(e *entry, limit int, window time.Duration) Result {
	now := time.Now()
	e.mu.Lock()

	// Initialize on first request
	if e.last.IsZero() {
		e.tokens = float64(limit - 1) // consume 1 token
		e.last = now
		e.windowStart = now
		e.mu.Unlock()
		return Result{
			Allowed:   true,
			Remaining: limit - 1,
			ResetAt:   now.Add(window),
		}
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(e.last)
	rate := float64(limit) / window.Seconds()
	e.tokens += elapsed.Seconds() * rate
	if e.tokens > float64(limit) {
		e.tokens = float64(limit)
	}
	e.last = now

	// Update window reset time
	if now.Sub(e.windowStart) >= window {
		e.windowStart = now
	}
	resetAt := e.windowStart.Add(window)

	if e.tokens < 1 {
		// Rejected — calculate retry time
		deficit := 1.0 - e.tokens
		retryAfter := time.Duration(deficit / rate * float64(time.Second))
		remaining := 0
		e.mu.Unlock()
		return Result{
			Allowed:   false,
			Remaining: remaining,
			ResetAt:   resetAt,
			RetryAt:   retryAfter,
		}
	}

	// Allowed — consume 1 token
	e.tokens--
	remaining := int(e.tokens)
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
