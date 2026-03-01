package kruda

import (
	"errors"
	"strings"
	"testing"
)

// moduleA registers a *testService into the container.
type moduleA struct{}

func (moduleA) Install(c *Container) error {
	return c.Give(&testService{Name: "from-module-a"})
}

// moduleB registers a *testServiceB into the container.
type moduleB struct{}

func (moduleB) Install(c *Container) error {
	return c.Give(&testServiceB{Value: 42})
}

// failModule always returns an error from Install.
type failModule struct{}

func (failModule) Install(c *Container) error {
	return errors.New("install failed")
}

// compositeModule installs moduleB inside its own Install.
type compositeModule struct{}

func (compositeModule) Install(c *Container) error {
	inner := moduleB{}
	if err := inner.Install(c); err != nil {
		return err
	}
	return c.Give(&testService{Name: "from-composite"})
}

func TestModuleInstall(t *testing.T) {
	app := New()
	app.container = NewContainer()
	app.Module(moduleA{})

	got, err := Use[*testService](app.container)
	if err != nil {
		t.Fatalf("Use failed: %v", err)
	}
	if got.Name != "from-module-a" {
		t.Fatalf("expected from-module-a, got %s", got.Name)
	}
}

func TestModuleAutoCreateContainer(t *testing.T) {
	app := New()
	// Ensure no container is set
	app.container = nil

	app.Module(moduleA{})

	if app.container == nil {
		t.Fatal("expected container to be auto-created")
	}

	got, err := Use[*testService](app.container)
	if err != nil {
		t.Fatalf("Use failed after auto-create: %v", err)
	}
	if got.Name != "from-module-a" {
		t.Fatalf("expected from-module-a, got %s", got.Name)
	}
}

func TestModulePanicOnError(t *testing.T) {
	app := New()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from failing module")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if !strings.Contains(msg, "module install failed") {
			t.Fatalf("panic message should mention module install failed, got: %s", msg)
		}
	}()

	app.Module(failModule{})
}

func TestModuleChaining(t *testing.T) {
	app := New()
	app.Module(moduleA{}).Module(moduleB{})

	gotA, err := Use[*testService](app.container)
	if err != nil {
		t.Fatalf("Use[*testService] failed: %v", err)
	}
	if gotA.Name != "from-module-a" {
		t.Fatalf("expected from-module-a, got %s", gotA.Name)
	}

	gotB, err := Use[*testServiceB](app.container)
	if err != nil {
		t.Fatalf("Use[*testServiceB] failed: %v", err)
	}
	if gotB.Value != 42 {
		t.Fatalf("expected 42, got %d", gotB.Value)
	}
}

func TestModuleComposition(t *testing.T) {
	app := New()
	app.Module(compositeModule{})

	gotSvc, err := Use[*testService](app.container)
	if err != nil {
		t.Fatalf("Use[*testService] failed: %v", err)
	}
	if gotSvc.Name != "from-composite" {
		t.Fatalf("expected from-composite, got %s", gotSvc.Name)
	}

	gotB, err := Use[*testServiceB](app.container)
	if err != nil {
		t.Fatalf("Use[*testServiceB] failed: %v", err)
	}
	if gotB.Value != 42 {
		t.Fatalf("expected 42, got %d", gotB.Value)
	}
}
