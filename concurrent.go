package kruda

import (
	"context"
	"errors"
	"sync"
)

// Parallel executes all tasks concurrently and returns a joined error of all failures.
// Waits for ALL tasks to complete before returning, even if some error early.
// Returns nil if all tasks succeed or if no tasks are provided.
// When multiple tasks fail, the returned error wraps all of them via errors.Join.
func Parallel(tasks ...func() error) error {
	if len(tasks) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errs := make([]error, len(tasks))

	wg.Add(len(tasks))
	for i, task := range tasks {
		go func(idx int, fn func() error) {
			defer wg.Done()
			errs[idx] = fn()
		}(i, task)
	}

	wg.Wait()
	return errors.Join(errs...)
}

// Race executes all tasks concurrently and returns the result of the first to complete.
// The provided context is passed to each task, allowing cancellation of slower tasks
// after the first one finishes. Callers should pass a cancellable context and defer
// its cancel function to clean up remaining goroutines.
// Returns (nil, nil) if no tasks are provided.
func Race(ctx context.Context, tasks ...func(context.Context) (any, error)) (any, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	type result struct {
		val any
		err error
	}
	ch := make(chan result, 1)

	for _, task := range tasks {
		go func(fn func(context.Context) (any, error)) {
			v, err := fn(ctx)
			// Non-blocking send — only first goroutine succeeds
			select {
			case ch <- result{v, err}:
			default:
			}
		}(task)
	}

	select {
	case r := <-ch:
		return r.val, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Each applies fn to each item concurrently and returns a joined error of all failures.
// Waits for ALL invocations to complete before returning.
// Returns nil if all succeed or if items is empty.
func Each[T any](items []T, fn func(T) error) error {
	if len(items) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errs := make([]error, len(items))

	wg.Add(len(items))
	for i, item := range items {
		go func(idx int, v T) {
			defer wg.Done()
			errs[idx] = fn(v)
		}(i, item)
	}

	wg.Wait()
	return errors.Join(errs...)
}
