package kruda

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Parallel executes all tasks concurrently and waits for ALL to complete.
// Returns a joined error of all failures, or nil if all succeed.
// A task that panics reports the panic as its error instead of crashing.
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
			errs[idx] = recoverTask(fn)
		}(i, task)
	}

	wg.Wait()
	return errors.Join(errs...)
}

// Race executes all tasks concurrently and returns the result of the first to complete.
// The context is passed to each task so callers can cancel slower goroutines.
// Returns (nil, nil) if no tasks are provided.
// A task that panics finishes with the panic as its error instead of crashing.
func Race(ctx context.Context, tasks ...func(context.Context) (any, error)) (any, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	type result struct {
		val any
		err error
	}
	ch := make(chan result, 1)
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, task := range tasks {
		go func(fn func(context.Context) (any, error)) {
			var v any
			err := recoverTask(func() (err error) {
				v, err = fn(raceCtx)
				return err
			})
			// Non-blocking send — only the first goroutine wins
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
// A task that panics reports the panic as its error instead of crashing.
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
			errs[idx] = recoverTask(func() error { return fn(v) })
		}(i, item)
	}

	wg.Wait()
	return errors.Join(errs...)
}

// recoverTask runs fn and turns a panic into an error. A panic in a goroutine
// cannot be caught by the request's Recovery middleware and would end the
// whole process.
func recoverTask(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("kruda: task panicked: %v", r)
		}
	}()
	return fn()
}
