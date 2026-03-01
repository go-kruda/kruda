package kruda

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/quick"
	"time"
)

func TestPropertyParallelSuccessAndError(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	t.Run("AllSuccess", func(t *testing.T) {
		f := func(n uint8) bool {
			count := int(n%10) + 1
			tasks := make([]func() error, count)
			for i := range tasks {
				tasks[i] = func() error { return nil }
			}
			return Parallel(tasks...) == nil
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("OneError", func(t *testing.T) {
		f := func(n uint8) bool {
			count := int(n%10) + 1
			sentinel := errors.New("fail")
			tasks := make([]func() error, count)
			for i := range tasks {
				if i == count/2 {
					tasks[i] = func() error { return sentinel }
				} else {
					tasks[i] = func() error { return nil }
				}
			}
			err := Parallel(tasks...)
			return err != nil && errors.Is(err, sentinel)
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("MultipleErrorsJoined", func(t *testing.T) {
		f := func(n uint8) bool {
			count := int(n%5) + 2
			sentinels := make([]error, count)
			tasks := make([]func() error, count)
			for i := range tasks {
				sentinels[i] = errors.New("fail")
				s := sentinels[i]
				tasks[i] = func() error { return s }
			}
			err := Parallel(tasks...)
			if err == nil {
				return false
			}
			for _, s := range sentinels {
				if !errors.Is(err, s) {
					return false
				}
			}
			return true
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})
}

func TestPropertyParallelWaitsForAll(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	f := func(n uint8) bool {
		count := int(n%8) + 1
		var completed atomic.Int32

		tasks := make([]func() error, count)
		for i := range tasks {
			tasks[i] = func() error {
				completed.Add(1)
				return nil
			}
		}

		_ = Parallel(tasks...)
		return int(completed.Load()) == count
	}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

func TestPropertyRaceReturnsFirstResult(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	t.Run("SingleTask", func(t *testing.T) {
		f := func(value int) bool {
			ctx := context.Background()
			val, err := Race(ctx, func(ctx context.Context) (any, error) {
				return value, nil
			})
			return err == nil && val == value
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("FastBeatsSlowDeterministic", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			val, err := Race(ctx,
				func(ctx context.Context) (any, error) {
					return "fast", nil
				},
				func(ctx context.Context) (any, error) {
					select {
					case <-time.After(50 * time.Millisecond):
						return "slow", nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				},
			)
			cancel()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if val != "fast" {
				t.Fatalf("val = %v, want fast", val)
			}
		}
	})
}

func TestPropertyEachSuccessAndError(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	t.Run("AllSuccess", func(t *testing.T) {
		f := func(items []int) bool {
			return Each(items, func(n int) error { return nil }) == nil
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("OneError", func(t *testing.T) {
		f := func(n uint8) bool {
			count := int(n%10) + 1
			items := make([]int, count)
			for i := range items {
				items[i] = i
			}
			errorAt := count / 2
			sentinel := errors.New("fail")
			err := Each(items, func(i int) error {
				if i == errorAt {
					return sentinel
				}
				return nil
			})
			return err != nil && errors.Is(err, sentinel)
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})
}

func TestPropertyEachWaitsForAll(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	f := func(items []uint8) bool {
		if len(items) == 0 {
			return true
		}
		// Cap to avoid too many goroutines
		if len(items) > 50 {
			items = items[:50]
		}
		var processed atomic.Int32
		_ = Each(items, func(v uint8) error {
			processed.Add(1)
			return nil
		})
		return int(processed.Load()) == len(items)
	}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}
