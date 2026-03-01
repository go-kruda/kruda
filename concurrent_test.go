package kruda

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallel_AllSuccess(t *testing.T) {
	var count atomic.Int32
	err := Parallel(
		func() error { count.Add(1); return nil },
		func() error { count.Add(1); return nil },
		func() error { count.Add(1); return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count.Load() != 3 {
		t.Errorf("count = %d, want 3", count.Load())
	}
}

func TestParallel_OneError(t *testing.T) {
	sentinel := errors.New("task failed")
	var count atomic.Int32
	err := Parallel(
		func() error { count.Add(1); return nil },
		func() error { count.Add(1); return sentinel },
		func() error { count.Add(1); return nil },
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error should wrap sentinel, got: %v", err)
	}
	// All tasks should complete even though one errored
	if count.Load() != 3 {
		t.Errorf("count = %d, want 3 (all tasks should complete)", count.Load())
	}
}

func TestParallel_MultipleErrors(t *testing.T) {
	err1 := errors.New("fail-1")
	err2 := errors.New("fail-2")
	err := Parallel(
		func() error { return err1 },
		func() error { return err2 },
		func() error { return nil },
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// errors.Join should wrap both errors
	if !errors.Is(err, err1) {
		t.Error("joined error should contain err1")
	}
	if !errors.Is(err, err2) {
		t.Error("joined error should contain err2")
	}
}

func TestParallel_ZeroTasks(t *testing.T) {
	err := Parallel()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParallel_WaitsForAll(t *testing.T) {
	var done atomic.Int32
	err := Parallel(
		func() error {
			time.Sleep(10 * time.Millisecond)
			done.Add(1)
			return nil
		},
		func() error {
			time.Sleep(20 * time.Millisecond)
			done.Add(1)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done.Load() != 2 {
		t.Errorf("done = %d, want 2", done.Load())
	}
}

func TestRace_SingleTask(t *testing.T) {
	ctx := context.Background()
	val, err := Race(ctx, func(ctx context.Context) (any, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Errorf("val = %v, want hello", val)
	}
}

func TestRace_FastAndSlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	val, err := Race(ctx,
		func(ctx context.Context) (any, error) {
			return "fast", nil // completes immediately
		},
		func(ctx context.Context) (any, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return "slow", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "fast" {
		t.Errorf("val = %v, want fast", val)
	}
}

func TestRace_ZeroTasks(t *testing.T) {
	val, err := Race(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("val = %v, want nil", val)
	}
}

func TestRace_FirstError(t *testing.T) {
	sentinel := errors.New("race error")
	val, err := Race(context.Background(), func(ctx context.Context) (any, error) {
		return nil, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want sentinel", err)
	}
	if val != nil {
		t.Errorf("val = %v, want nil", val)
	}
}

func TestRace_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	val, err := Race(ctx, func(ctx context.Context) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if val != nil {
		t.Errorf("val = %v, want nil", val)
	}
}

func TestEach_AllSuccess(t *testing.T) {
	var sum atomic.Int32
	items := []int{1, 2, 3, 4, 5}
	err := Each(items, func(n int) error {
		sum.Add(int32(n))
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.Load() != 15 {
		t.Errorf("sum = %d, want 15", sum.Load())
	}
}

func TestEach_OneError(t *testing.T) {
	sentinel := errors.New("item error")
	var count atomic.Int32
	items := []int{1, 2, 3}
	err := Each(items, func(n int) error {
		count.Add(1)
		if n == 2 {
			return sentinel
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error should wrap sentinel, got: %v", err)
	}
	// All items should be processed
	if count.Load() != 3 {
		t.Errorf("count = %d, want 3", count.Load())
	}
}

func TestEach_MultipleErrors(t *testing.T) {
	err1 := errors.New("fail-a")
	err2 := errors.New("fail-b")
	items := []int{1, 2, 3}
	err := Each(items, func(n int) error {
		if n == 1 {
			return err1
		}
		if n == 3 {
			return err2
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, err1) {
		t.Error("joined error should contain err1")
	}
	if !errors.Is(err, err2) {
		t.Error("joined error should contain err2")
	}
}

func TestEach_EmptySlice(t *testing.T) {
	err := Each([]int{}, func(n int) error {
		t.Fatal("should not be called")
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEach_StringItems(t *testing.T) {
	var results atomic.Int32
	items := []string{"a", "b", "c"}
	err := Each(items, func(s string) error {
		results.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results.Load() != 3 {
		t.Errorf("results = %d, want 3", results.Load())
	}
}
