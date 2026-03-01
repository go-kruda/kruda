package kruda

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/quick"
)

// PBT wrapper types — each test needs a unique type because Give keys by reflect.Type.

// pbtSingletonWrapper wraps a value for singleton tests.
type pbtSingletonWrapper struct{ Val int }

type pbtTransientWrapper struct{ Val int }

type pbtLazyWrapper struct{ Val int }

type pbtNamedWrapper struct{ Val int }

type pbtDupWrapper struct{ Val int }

type pbtConcSingletonWrapper struct{ Val int }

type pbtConcLazyWrapper struct{ Val int }

type pbtConcTransientWrapper struct{ Val int }

// pbtLifecycleWrapper tracks init/shutdown calls for ordering tests.
type pbtLifecycleWrapper struct {
	id      int
	initLog *[]int
	shutLog *[]int
	mu      *sync.Mutex
}

func (w *pbtLifecycleWrapper) OnInit(_ context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	*w.initLog = append(*w.initLog, w.id)
	return nil
}

func (w *pbtLifecycleWrapper) OnShutdown(_ context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	*w.shutLog = append(*w.shutLog, w.id)
	return nil
}

func TestPropertySingletonRoundTrip(t *testing.T) {
	f := func(val int) bool {
		c := NewContainer()
		w := &pbtSingletonWrapper{Val: val}
		if err := c.Give(w); err != nil {
			return false
		}
		got, err := Use[*pbtSingletonWrapper](c)
		if err != nil {
			return false
		}
		// Same pointer and same value
		return got == w && got.Val == val
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestPropertyTransientDistinct(t *testing.T) {
	f := func(n uint8) bool {
		count := int(n)%9 + 2 // 2-10
		c := NewContainer()
		err := c.GiveTransient(func() (*pbtTransientWrapper, error) {
			return &pbtTransientWrapper{}, nil
		})
		if err != nil {
			return false
		}

		seen := make(map[*pbtTransientWrapper]bool, count)
		for i := 0; i < count; i++ {
			got, err := Use[*pbtTransientWrapper](c)
			if err != nil {
				return false
			}
			if seen[got] {
				return false // duplicate pointer — not distinct
			}
			seen[got] = true
		}
		return len(seen) == count
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestPropertyLazySingletonOnce(t *testing.T) {
	f := func(n uint8) bool {
		count := int(n)%10 + 1 // 1-10
		var callCount atomic.Int64
		c := NewContainer()
		err := c.GiveLazy(func() (*pbtLazyWrapper, error) {
			callCount.Add(1)
			return &pbtLazyWrapper{Val: 42}, nil
		})
		if err != nil {
			return false
		}

		var first *pbtLazyWrapper
		for i := 0; i < count; i++ {
			got, err := Use[*pbtLazyWrapper](c)
			if err != nil {
				return false
			}
			if i == 0 {
				first = got
			} else if got != first {
				return false // different pointer
			}
		}
		return callCount.Load() == 1
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestPropertyNamedRoundTrip(t *testing.T) {
	f := func(name string) bool {
		if name == "" {
			name = "default" // avoid empty name edge case
		}
		c := NewContainer()
		w := &pbtNamedWrapper{Val: len(name)}
		if err := c.GiveNamed(name, w); err != nil {
			return false
		}
		got, err := UseNamed[*pbtNamedWrapper](c, name)
		if err != nil {
			return false
		}
		return got == w && got.Val == len(name)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestPropertyDuplicateRejected(t *testing.T) {
	f := func(val1, val2 int) bool {
		c := NewContainer()
		w1 := &pbtDupWrapper{Val: val1}
		w2 := &pbtDupWrapper{Val: val2}

		if err := c.Give(w1); err != nil {
			return false
		}
		// Second Give with same type must fail
		err := c.Give(w2)
		if err == nil {
			return false
		}
		// First registration should remain intact
		got, err := Use[*pbtDupWrapper](c)
		if err != nil {
			return false
		}
		return got == w1 && got.Val == val1
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestPropertyConcurrentSingletonSame(t *testing.T) {
	f := func(n uint8) bool {
		goroutines := int(n)%50 + 2 // 2-51
		c := NewContainer()
		w := &pbtConcSingletonWrapper{Val: 99}
		if err := c.Give(w); err != nil {
			return false
		}

		var wg sync.WaitGroup
		results := make([]*pbtConcSingletonWrapper, goroutines)
		errs := make([]error, goroutines)
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func(idx int) {
				defer wg.Done()
				results[idx], errs[idx] = Use[*pbtConcSingletonWrapper](c)
			}(i)
		}
		wg.Wait()

		for i := 0; i < goroutines; i++ {
			if errs[i] != nil {
				return false
			}
			if results[i] != w {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestPropertyConcurrentLazyOnce(t *testing.T) {
	f := func(n uint8) bool {
		goroutines := int(n)%50 + 2 // 2-51
		var callCount atomic.Int64
		c := NewContainer()
		err := c.GiveLazy(func() (*pbtConcLazyWrapper, error) {
			callCount.Add(1)
			return &pbtConcLazyWrapper{Val: 77}, nil
		})
		if err != nil {
			return false
		}

		var wg sync.WaitGroup
		results := make([]*pbtConcLazyWrapper, goroutines)
		errs := make([]error, goroutines)
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func(idx int) {
				defer wg.Done()
				results[idx], errs[idx] = Use[*pbtConcLazyWrapper](c)
			}(i)
		}
		wg.Wait()

		// Factory called exactly once
		if callCount.Load() != 1 {
			return false
		}
		// All results are the same instance
		for i := 0; i < goroutines; i++ {
			if errs[i] != nil {
				return false
			}
			if results[i] != results[0] {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestPropertyConcurrentTransientDistinct(t *testing.T) {
	f := func(n uint8) bool {
		goroutines := int(n)%19 + 2 // 2-20
		c := NewContainer()
		err := c.GiveTransient(func() (*pbtConcTransientWrapper, error) {
			return &pbtConcTransientWrapper{}, nil
		})
		if err != nil {
			return false
		}

		var wg sync.WaitGroup
		results := make([]*pbtConcTransientWrapper, goroutines)
		errs := make([]error, goroutines)
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func(idx int) {
				defer wg.Done()
				results[idx], errs[idx] = Use[*pbtConcTransientWrapper](c)
			}(i)
		}
		wg.Wait()

		seen := make(map[*pbtConcTransientWrapper]bool, goroutines)
		for i := 0; i < goroutines; i++ {
			if errs[i] != nil {
				return false
			}
			if seen[results[i]] {
				return false // duplicate pointer
			}
			seen[results[i]] = true
		}
		return len(seen) == goroutines
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestPropertyLifecycleOrdering(t *testing.T) {
	f := func(n uint8) bool {
		count := int(n)%10 + 1 // 1-10
		initLog := make([]int, 0, count)
		shutLog := make([]int, 0, count)
		var mu sync.Mutex

		c := NewContainer()
		for i := 0; i < count; i++ {
			w := &pbtLifecycleWrapper{
				id:      i,
				initLog: &initLog,
				shutLog: &shutLog,
				mu:      &mu,
			}
			// Use GiveNamed to register multiple instances of the same type.
			// GiveNamed adds to initOrder, so lifecycle hooks will fire.
			key := fmt.Sprintf("lifecycle-%d", i)
			if err := c.GiveNamed(key, w); err != nil {
				return false
			}
		}

		// Start should call OnInit in registration order
		if err := c.Start(context.Background()); err != nil {
			return false
		}
		mu.Lock()
		if len(initLog) != count {
			mu.Unlock()
			return false
		}
		for i := 0; i < count; i++ {
			if initLog[i] != i {
				mu.Unlock()
				return false
			}
		}
		mu.Unlock()

		// Shutdown should call OnShutdown in reverse order
		if err := c.Shutdown(context.Background()); err != nil {
			return false
		}
		mu.Lock()
		defer mu.Unlock()
		if len(shutLog) != count {
			return false
		}
		for i := 0; i < count; i++ {
			expected := count - 1 - i
			if shutLog[i] != expected {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// pbtModuleService carries an ID to verify ordering.
type pbtModuleService struct {
	ID int
}

// pbtModule registers a sequence of named services into the container.
type pbtModule struct {
	services []*pbtModuleService
}

func (m *pbtModule) Install(c *Container) error {
	for _, svc := range m.services {
		key := fmt.Sprintf("mod-svc-%d", svc.ID)
		if err := c.GiveNamed(key, svc); err != nil {
			return err
		}
	}
	return nil
}

func TestPropertyModuleRegistrationOrder(t *testing.T) {
	f := func(nModules, nPerModule uint8) bool {
		modules := int(nModules)%5 + 1     // 1-5 modules
		perModule := int(nPerModule)%4 + 1 // 1-4 services per module

		app := New()
		globalID := 0
		expectedOrder := make([]int, 0, modules*perModule)

		for m := 0; m < modules; m++ {
			mod := &pbtModule{services: make([]*pbtModuleService, perModule)}
			for s := 0; s < perModule; s++ {
				mod.services[s] = &pbtModuleService{ID: globalID}
				expectedOrder = append(expectedOrder, globalID)
				globalID++
			}
			app.Module(mod)
		}

		// Verify initOrder matches expected order
		app.container.mu.RLock()
		order := make([]any, len(app.container.initOrder))
		copy(order, app.container.initOrder)
		app.container.mu.RUnlock()

		if len(order) != len(expectedOrder) {
			return false
		}
		for i, inst := range order {
			svc, ok := inst.(*pbtModuleService)
			if !ok {
				return false
			}
			if svc.ID != expectedOrder[i] {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
