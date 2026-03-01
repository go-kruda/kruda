package middleware

import (
	"context"
	"time"

	"github.com/go-kruda/kruda"
)

// Timeout returns middleware that sets a deadline on the request context.
// If the handler's context-aware operations exceed the specified duration,
// they will receive a context.DeadlineExceeded error.
//
// The handler runs synchronously (no goroutine) to avoid data races on Ctx,
// use-after-free from pool reuse, and goroutine leaks. The timeout is enforced
// via context cancellation — handlers should check c.Context().Done() or pass
// c.Context() to I/O operations.
//
// If the handler returns and the context deadline has been exceeded,
// a 503 Service Unavailable response is returned.
func Timeout(duration time.Duration) kruda.HandlerFunc {
	return func(c *kruda.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), duration)
		defer cancel()
		c.SetContext(ctx)

		err := c.Next()

		if ctx.Err() == context.DeadlineExceeded {
			if err != nil {
				c.Log().Warn("handler error discarded due to timeout",
					"original_error", err.Error(),
				)
			}
			return kruda.NewError(503, "service unavailable: request timeout")
		}

		return err
	}
}
