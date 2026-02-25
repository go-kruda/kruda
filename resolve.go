package kruda

import "errors"

// resolveContainer returns the DI container from the request context.
// Checks ctx locals first (InjectMiddleware), then app-level container.
func resolveContainer(c *Ctx) (*Container, error) {
	if raw := c.Get("container"); raw != nil {
		container, ok := raw.(*Container)
		if !ok {
			return nil, errors.New("kruda: invalid container in context")
		}
		return container, nil
	}
	if c.app.container != nil {
		return c.app.container, nil
	}
	return nil, errors.New("kruda: no container configured")
}

// Resolve resolves a service of type T from the DI container
// attached to the current request context.
//
// Resolution order:
//  1. Check ctx locals for "container" key (set by InjectMiddleware)
//  2. Fallback to app-level container (set by WithContainer option)
//  3. Return error if neither is configured
func Resolve[T any](c *Ctx) (T, error) {
	var zero T
	container, err := resolveContainer(c)
	if err != nil {
		return zero, err
	}
	return Use[T](container)
}

// ResolveNamed resolves a named instance of type T from the DI container
// attached to the current request context. Same resolution order as Resolve.
func ResolveNamed[T any](c *Ctx, name string) (T, error) {
	var zero T
	container, err := resolveContainer(c)
	if err != nil {
		return zero, err
	}
	return UseNamed[T](container, name)
}

// MustResolve is like Resolve but panics on error.
// Useful in handlers where a missing service is a programming error.
func MustResolve[T any](c *Ctx) T {
	v, err := Resolve[T](c)
	if err != nil {
		panic(err)
	}
	return v
}

// MustResolveNamed is like ResolveNamed but panics on error.
func MustResolveNamed[T any](c *Ctx, name string) T {
	v, err := ResolveNamed[T](c, name)
	if err != nil {
		panic(err)
	}
	return v
}
