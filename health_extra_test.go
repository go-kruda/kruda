package kruda

import (
	"context"
	"testing"
)

// =============================================================================
// health.go — discoverHealthCheckers with lazy + named instances
// =============================================================================

type namedHealthService struct {
	name string
}

func (s *namedHealthService) Check(_ context.Context) error { return nil }

type nonComparableHealthService []string

func (nonComparableHealthService) Check(_ context.Context) error { return nil }

func TestDiscoverHealthCheckers_WithLazySingleton(t *testing.T) {
	c := NewContainer()
	_ = c.GiveLazy(func() (*healthyDB, error) {
		return &healthyDB{}, nil
	})
	// Resolve the lazy to make it "done"
	_, _ = Use[*healthyDB](c)

	checkers := discoverHealthCheckers(c)
	found := false
	for _, ch := range checkers {
		if ch.checker != nil {
			found = true
		}
	}
	if !found {
		t.Error("discoverHealthCheckers should find resolved lazy singletons")
	}
}

func TestDiscoverHealthCheckers_WithUnresolvedLazy(t *testing.T) {
	c := NewContainer()
	_ = c.GiveLazy(func() (*healthyDB, error) {
		return &healthyDB{}, nil
	})
	// Don't resolve — lazy is not "done"

	checkers := discoverHealthCheckers(c)
	for _, ch := range checkers {
		if _, ok := ch.checker.(*healthyDB); ok {
			t.Error("discoverHealthCheckers should NOT find unresolved lazy singletons")
		}
	}
}

func TestDiscoverHealthCheckers_WithNamedInstance(t *testing.T) {
	c := NewContainer()
	svc := &namedHealthService{name: "primary-db"}
	_ = c.GiveNamed("primary-db", svc)

	checkers := discoverHealthCheckers(c)
	found := false
	for _, ch := range checkers {
		if ch.name == "primary-db" {
			found = true
		}
	}
	if !found {
		t.Error("discoverHealthCheckers should find named health checkers")
	}
}

func TestDiscoverHealthCheckers_NilContainer(t *testing.T) {
	checkers := discoverHealthCheckers(nil)
	if checkers != nil {
		t.Error("discoverHealthCheckers(nil) should return nil")
	}
}

func TestDiscoverHealthCheckers_DedupSameInstance(t *testing.T) {
	c := NewContainer()
	svc := &healthyDB{}
	_ = c.Give(svc)
	// Register the same instance under a name
	_ = c.GiveNamed("db", svc)

	checkers := discoverHealthCheckers(c)
	count := 0
	for _, ch := range checkers {
		if _, ok := ch.checker.(*healthyDB); ok {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected dedup, got %d checkers for same instance", count)
	}
}

func TestDiscoverHealthCheckers_NonHealthChecker(t *testing.T) {
	c := NewContainer()
	type plainService struct{ Name string }
	_ = c.Give(&plainService{Name: "plain"})

	checkers := discoverHealthCheckers(c)
	if len(checkers) != 0 {
		t.Errorf("expected 0 checkers for non-HealthChecker, got %d", len(checkers))
	}
}

func TestDiscoverHealthCheckers_NonComparableServices(t *testing.T) {
	c := NewContainer()
	if err := c.Give(nonComparableHealthService{"worker", "scheduler"}); err != nil {
		t.Fatal(err)
	}
	if err := c.GiveNamed("metadata", map[string]string{"region": "local"}); err != nil {
		t.Fatal(err)
	}

	checkers := discoverHealthCheckers(c)
	if len(checkers) != 1 {
		t.Fatalf("expected the non-comparable health service only, got %d checkers", len(checkers))
	}
	if _, ok := checkers[0].checker.(nonComparableHealthService); !ok {
		t.Fatalf("checker type = %T, want nonComparableHealthService", checkers[0].checker)
	}
}

func TestDiscoverHealthCheckers_NonComparableLazyInterfaceService(t *testing.T) {
	c := NewContainer()
	if err := c.GiveLazy(func() (HealthChecker, error) {
		return nonComparableHealthService{"queue"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Use[HealthChecker](c); err != nil {
		t.Fatal(err)
	}

	checkers := discoverHealthCheckers(c)
	if len(checkers) != 1 {
		t.Fatalf("expected the resolved lazy health service, got %d checkers", len(checkers))
	}
}
