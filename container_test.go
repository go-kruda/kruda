package kruda

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// Test helpers — types used across multiple tests
// ---------------------------------------------------------------------------

type testService struct {
	Name string
}

type testServiceB struct {
	Value int
}

type testInterface interface {
	Ping() string
}

type testImpl struct {
	msg string
}

func (t *testImpl) Ping() string { return t.msg }

// lifecycleService tracks OnInit/OnShutdown calls for ordering tests.
// Each distinct type is needed because Give() keys by reflect.Type.
type lifecycleService struct {
	name    string
	initLog *[]string
	shutLog *[]string
	initErr error
}

func (s *lifecycleService) OnInit(ctx context.Context) error {
	if s.initLog != nil {
		*s.initLog = append(*s.initLog, s.name)
	}
	return s.initErr
}

func (s *lifecycleService) OnShutdown(ctx context.Context) error {
	if s.shutLog != nil {
		*s.shutLog = append(*s.shutLog, s.name)
	}
	return nil
}

// Distinct wrapper types so each can be registered separately via Give()
// (Give keys by reflect.Type — same concrete type = duplicate error).
type lifecycleA struct{ lifecycleService }
type lifecycleB struct{ lifecycleService }
type lifecycleC struct{ lifecycleService }

// ---------------------------------------------------------------------------
// 1. TestContainerGiveAndUse — singleton round-trip, pointer equality
// ---------------------------------------------------------------------------

func TestContainerGiveAndUse(t *testing.T) {
	c := NewContainer()
	svc := &testService{Name: "hello"}

	if err := c.Give(svc); err != nil {
		t.Fatalf("Give failed: %v", err)
	}

	got, err := Use[*testService](c)
	if err != nil {
		t.Fatalf("Use failed: %v", err)
	}
	if got != svc {
		t.Fatalf("expected same pointer, got different instance")
	}
	if got.Name != "hello" {
		t.Fatalf("expected Name=hello, got %s", got.Name)
	}
}

// ---------------------------------------------------------------------------
// 2. TestContainerGiveAs — register by interface, resolve by interface type
// ---------------------------------------------------------------------------

func TestContainerGiveAs(t *testing.T) {
	c := NewContainer()
	impl := &testImpl{msg: "pong"}

	if err := c.GiveAs(impl, (*testInterface)(nil)); err != nil {
		t.Fatalf("GiveAs failed: %v", err)
	}

	got, err := Use[testInterface](c)
	if err != nil {
		t.Fatalf("Use[testInterface] failed: %v", err)
	}
	if got.Ping() != "pong" {
		t.Fatalf("expected pong, got %s", got.Ping())
	}
}

// ---------------------------------------------------------------------------
// 3. TestContainerGiveTransient — different instances on each resolve
// ---------------------------------------------------------------------------

func TestContainerGiveTransient(t *testing.T) {
	c := NewContainer()
	var callCount atomic.Int64

	err := c.GiveTransient(func() (*testService, error) {
		callCount.Add(1)
		return &testService{Name: "transient"}, nil
	})
	if err != nil {
		t.Fatalf("GiveTransient failed: %v", err)
	}

	a, err := Use[*testService](c)
	if err != nil {
		t.Fatalf("first Use failed: %v", err)
	}
	b, err := Use[*testService](c)
	if err != nil {
		t.Fatalf("second Use failed: %v", err)
	}

	if a == b {
		t.Fatal("transient should return different instances")
	}
	if callCount.Load() != 2 {
		t.Fatalf("factory should be called twice, got %d", callCount.Load())
	}
}

// ---------------------------------------------------------------------------
// 4. TestContainerGiveLazy — same instance, factory called once
// ---------------------------------------------------------------------------

func TestContainerGiveLazy(t *testing.T) {
	c := NewContainer()
	var callCount atomic.Int64

	err := c.GiveLazy(func() (*testService, error) {
		callCount.Add(1)
		return &testService{Name: "lazy"}, nil
	})
	if err != nil {
		t.Fatalf("GiveLazy failed: %v", err)
	}

	a, err := Use[*testService](c)
	if err != nil {
		t.Fatalf("first Use failed: %v", err)
	}
	b, err := Use[*testService](c)
	if err != nil {
		t.Fatalf("second Use failed: %v", err)
	}

	if a != b {
		t.Fatal("lazy singleton should return same instance")
	}
	if callCount.Load() != 1 {
		t.Fatalf("factory should be called once, got %d", callCount.Load())
	}
}

// ---------------------------------------------------------------------------
// 5. TestContainerGiveLazyRetry — factory fails first, succeeds second (R2.8)
// ---------------------------------------------------------------------------

func TestContainerGiveLazyRetry(t *testing.T) {
	c := NewContainer()
	var attempt atomic.Int64

	err := c.GiveLazy(func() (*testService, error) {
		n := attempt.Add(1)
		if n == 1 {
			return nil, errors.New("temporary failure")
		}
		return &testService{Name: "recovered"}, nil
	})
	if err != nil {
		t.Fatalf("GiveLazy failed: %v", err)
	}

	// First call should fail
	_, err = Use[*testService](c)
	if err == nil {
		t.Fatal("first Use should fail")
	}
	if !strings.Contains(err.Error(), "temporary failure") {
		t.Fatalf("expected temporary failure in error, got: %v", err)
	}

	// Second call should succeed (retry)
	got, err := Use[*testService](c)
	if err != nil {
		t.Fatalf("second Use should succeed: %v", err)
	}
	if got.Name != "recovered" {
		t.Fatalf("expected recovered, got %s", got.Name)
	}

	// Third call should return cached instance
	got2, err := Use[*testService](c)
	if err != nil {
		t.Fatalf("third Use failed: %v", err)
	}
	if got != got2 {
		t.Fatal("after success, lazy should return cached instance")
	}
	if attempt.Load() != 2 {
		t.Fatalf("factory should be called twice (1 fail + 1 success), got %d", attempt.Load())
	}
}

// ---------------------------------------------------------------------------
// 6. TestContainerGiveNamed — register and resolve by name
// ---------------------------------------------------------------------------

func TestContainerGiveNamed(t *testing.T) {
	c := NewContainer()
	writeDB := &testService{Name: "write-db"}
	readDB := &testService{Name: "read-db"}

	if err := c.GiveNamed("write", writeDB); err != nil {
		t.Fatalf("GiveNamed write failed: %v", err)
	}
	if err := c.GiveNamed("read", readDB); err != nil {
		t.Fatalf("GiveNamed read failed: %v", err)
	}

	gotW, err := UseNamed[*testService](c, "write")
	if err != nil {
		t.Fatalf("UseNamed write failed: %v", err)
	}
	if gotW != writeDB {
		t.Fatal("expected same write-db pointer")
	}

	gotR, err := UseNamed[*testService](c, "read")
	if err != nil {
		t.Fatalf("UseNamed read failed: %v", err)
	}
	if gotR != readDB {
		t.Fatal("expected same read-db pointer")
	}
}

// ---------------------------------------------------------------------------
// 7. TestContainerGiveNil — error on nil registration
// ---------------------------------------------------------------------------

func TestContainerGiveNil(t *testing.T) {
	c := NewContainer()

	err := c.Give(nil)
	if err == nil {
		t.Fatal("Give(nil) should return error")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Fatalf("error should mention nil, got: %v", err)
	}

	err = c.GiveNamed("x", nil)
	if err == nil {
		t.Fatal("GiveNamed(nil) should return error")
	}

	err = c.GiveAs(nil, (*testInterface)(nil))
	if err == nil {
		t.Fatal("GiveAs(nil) should return error")
	}
}

// ---------------------------------------------------------------------------
// 8. TestContainerGiveDuplicate — error on duplicate type registration
// ---------------------------------------------------------------------------

func TestContainerGiveDuplicate(t *testing.T) {
	c := NewContainer()
	svc1 := &testService{Name: "first"}
	svc2 := &testService{Name: "second"}

	if err := c.Give(svc1); err != nil {
		t.Fatalf("first Give failed: %v", err)
	}

	err := c.Give(svc2)
	if err == nil {
		t.Fatal("second Give should return duplicate error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error should mention duplicate, got: %v", err)
	}

	// Verify first registration is intact
	got, err := Use[*testService](c)
	if err != nil {
		t.Fatalf("Use after duplicate should work: %v", err)
	}
	if got != svc1 {
		t.Fatal("first registration should remain intact")
	}

	// Duplicate named
	if err := c.GiveNamed("db", svc1); err != nil {
		t.Fatalf("first GiveNamed failed: %v", err)
	}
	err = c.GiveNamed("db", svc2)
	if err == nil {
		t.Fatal("duplicate GiveNamed should return error")
	}
}

// ---------------------------------------------------------------------------
// 9. TestContainerUseNotFound — error on unregistered type
// ---------------------------------------------------------------------------

func TestContainerUseNotFound(t *testing.T) {
	c := NewContainer()

	_, err := Use[*testService](c)
	if err == nil {
		t.Fatal("Use on empty container should return error")
	}
	if !strings.Contains(err.Error(), "no provider") {
		t.Fatalf("error should mention no provider, got: %v", err)
	}

	_, err = UseNamed[*testService](c, "missing")
	if err == nil {
		t.Fatal("UseNamed for missing name should return error")
	}
	if !strings.Contains(err.Error(), "no named instance") {
		t.Fatalf("error should mention no named instance, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 10. TestContainerCircularDependency — cycle detection with chain message
// ---------------------------------------------------------------------------

func TestContainerCircularDependency(t *testing.T) {
	c := NewContainer()

	// Register transient factories that create a cycle:
	// *testService factory calls Use[*testServiceB] which calls Use[*testService]
	err := c.GiveTransient(func() (*testService, error) {
		_, err := Use[*testServiceB](c)
		if err != nil {
			return nil, err
		}
		return &testService{Name: "a"}, nil
	})
	if err != nil {
		t.Fatalf("GiveTransient A failed: %v", err)
	}

	err = c.GiveTransient(func() (*testServiceB, error) {
		_, err := Use[*testService](c)
		if err != nil {
			return nil, err
		}
		return &testServiceB{Value: 1}, nil
	})
	if err != nil {
		t.Fatalf("GiveTransient B failed: %v", err)
	}

	// Resolving should detect the cycle
	_, err = Use[*testService](c)
	if err == nil {
		t.Fatal("circular dependency should return error")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("error should mention circular dependency, got: %v", err)
	}
	// The chain should contain the arrow separator
	if !strings.Contains(err.Error(), "→") {
		t.Fatalf("error should contain chain with →, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 11. TestContainerLifecycleStart — OnInit called in registration order
// ---------------------------------------------------------------------------

func TestContainerLifecycleStart(t *testing.T) {
	c := NewContainer()
	var initLog []string
	var shutLog []string

	svcA := &lifecycleA{lifecycleService{name: "A", initLog: &initLog, shutLog: &shutLog}}
	svcB := &lifecycleB{lifecycleService{name: "B", initLog: &initLog, shutLog: &shutLog}}
	svcC := &lifecycleC{lifecycleService{name: "C", initLog: &initLog, shutLog: &shutLog}}

	c.Give(svcA)
	c.Give(svcB)
	c.Give(svcC)

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	expected := []string{"A", "B", "C"}
	if len(initLog) != len(expected) {
		t.Fatalf("expected %d inits, got %d: %v", len(expected), len(initLog), initLog)
	}
	for i, name := range expected {
		if initLog[i] != name {
			t.Fatalf("init[%d] expected %s, got %s", i, name, initLog[i])
		}
	}
}

// ---------------------------------------------------------------------------
// 12. TestContainerLifecycleStartFailure — cleanup on OnInit failure
// ---------------------------------------------------------------------------

func TestContainerLifecycleStartFailure(t *testing.T) {
	c := NewContainer()
	var initLog []string
	var shutLog []string

	svcA := &lifecycleA{lifecycleService{name: "A", initLog: &initLog, shutLog: &shutLog}}
	svcB := &lifecycleB{lifecycleService{name: "B", initLog: &initLog, shutLog: &shutLog, initErr: fmt.Errorf("B init failed")}}
	svcC := &lifecycleC{lifecycleService{name: "C", initLog: &initLog, shutLog: &shutLog}}

	c.Give(svcA)
	c.Give(svcB)
	c.Give(svcC)

	err := c.Start(context.Background())
	if err == nil {
		t.Fatal("Start should fail when OnInit fails")
	}
	if !strings.Contains(err.Error(), "B init failed") {
		t.Fatalf("error should contain B init failed, got: %v", err)
	}

	// A should have been initialized, B failed, C never reached
	if len(initLog) != 2 {
		t.Fatalf("expected 2 inits (A, B), got %d: %v", len(initLog), initLog)
	}

	// A should have been cleaned up (OnShutdown called)
	if len(shutLog) != 1 || shutLog[0] != "A" {
		t.Fatalf("expected shutdown of A only, got: %v", shutLog)
	}
}

// ---------------------------------------------------------------------------
// 13. TestContainerLifecycleShutdown — OnShutdown in reverse order
// ---------------------------------------------------------------------------

func TestContainerLifecycleShutdown(t *testing.T) {
	c := NewContainer()
	var shutLog []string

	svcA := &lifecycleA{lifecycleService{name: "A", shutLog: &shutLog}}
	svcB := &lifecycleB{lifecycleService{name: "B", shutLog: &shutLog}}
	svcC := &lifecycleC{lifecycleService{name: "C", shutLog: &shutLog}}

	c.Give(svcA)
	c.Give(svcB)
	c.Give(svcC)

	if err := c.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	expected := []string{"C", "B", "A"}
	if len(shutLog) != len(expected) {
		t.Fatalf("expected %d shutdowns, got %d: %v", len(expected), len(shutLog), shutLog)
	}
	for i, name := range expected {
		if shutLog[i] != name {
			t.Fatalf("shutdown[%d] expected %s, got %s", i, name, shutLog[i])
		}
	}
}

// ---------------------------------------------------------------------------
// 14. TestContainerConcurrentUse — concurrent singleton resolution (race detector)
// ---------------------------------------------------------------------------

func TestContainerConcurrentUse(t *testing.T) {
	c := NewContainer()
	svc := &testService{Name: "shared"}
	if err := c.Give(svc); err != nil {
		t.Fatalf("Give failed: %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	results := make([]*testService, goroutines)
	errs := make([]error, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = Use[*testService](c)
		}(i)
	}
	wg.Wait()

	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d failed: %v", i, errs[i])
		}
		if results[i] != svc {
			t.Fatalf("goroutine %d got different instance", i)
		}
	}
}

// ---------------------------------------------------------------------------
// 15. TestContainerUseNamedTypeMismatch — error when type doesn't match
// ---------------------------------------------------------------------------

func TestContainerUseNamedTypeMismatch(t *testing.T) {
	c := NewContainer()
	svc := &testService{Name: "hello"}
	if err := c.GiveNamed("svc", svc); err != nil {
		t.Fatalf("GiveNamed failed: %v", err)
	}

	// Try to resolve as wrong type
	_, err := UseNamed[*testServiceB](c, "svc")
	if err == nil {
		t.Fatal("UseNamed with wrong type should return error")
	}
	if !strings.Contains(err.Error(), "testService") {
		t.Fatalf("error should mention actual type, got: %v", err)
	}
}
