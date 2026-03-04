package kruda

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Container is the dependency injection container.
// Stores services keyed by reflect.Type with three lifetime models:
// singleton (Give), transient (GiveTransient), and lazy singleton (GiveLazy).
// Thread-safe via sync.RWMutex.
type Container struct {
	mu         sync.RWMutex
	singletons map[reflect.Type]any
	transients map[reflect.Type]*transientEntry
	lazies     map[reflect.Type]*lazyEntry
	named      map[string]any
	initOrder  []any
	resolving  sync.Map // goroutine ID → []reflect.Type stack for cycle detection
}

// transientEntry holds a factory function for transient services.
type transientEntry struct {
	factory func() (any, error)
}

// lazyEntry holds a lazy singleton factory.
// Uses sync.Mutex (not sync.Once) to allow retry on factory failure.
// The done field uses atomic.Bool for lock-free reads in discoverHealthCheckers.
type lazyEntry struct {
	factory  func() (any, error)
	mu       sync.Mutex
	instance any
	done     atomic.Bool
}

// NewContainer creates a new empty DI container.
func NewContainer() *Container {
	return &Container{
		singletons: make(map[reflect.Type]any),
		transients: make(map[reflect.Type]*transientEntry),
		lazies:     make(map[reflect.Type]*lazyEntry),
		named:      make(map[string]any),
	}
}

// Give registers a singleton instance keyed by its reflect.Type.
// Returns error if instance is nil or type already registered.
func (c *Container) Give(instance any) error {
	if instance == nil {
		return errors.New("kruda: cannot register nil value")
	}
	t := reflect.TypeOf(instance)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isRegistered(t) {
		return fmt.Errorf("kruda: duplicate registration for type %v", t)
	}
	c.singletons[t] = instance
	c.initOrder = append(c.initOrder, instance)
	return nil
}

// GiveAs registers a singleton keyed by the interface type extracted from ifacePtr.
// ifacePtr must be a pointer to an interface (e.g. (*MyInterface)(nil)).
func (c *Container) GiveAs(instance any, ifacePtr any) error {
	if instance == nil {
		return errors.New("kruda: cannot register nil value")
	}
	if ifacePtr == nil {
		return errors.New("kruda: ifacePtr must be a non-nil pointer to an interface")
	}
	t := reflect.TypeOf(ifacePtr)
	if t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Interface {
		return errors.New("kruda: ifacePtr must be a pointer to an interface (e.g. (*MyInterface)(nil))")
	}
	ifaceType := t.Elem()
	if !reflect.TypeOf(instance).Implements(ifaceType) {
		return fmt.Errorf("kruda: %T does not implement %v", instance, ifaceType)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isRegistered(ifaceType) {
		return fmt.Errorf("kruda: duplicate registration for type %v", ifaceType)
	}
	c.singletons[ifaceType] = instance
	c.initOrder = append(c.initOrder, instance)
	return nil
}

// validateFactory checks that a factory function has the correct signature:
// func() T or func() (T, error). Returns the return type or an error.
func validateFactory(factory any) (reflect.Type, error) {
	ft := reflect.TypeOf(factory)
	if ft == nil || ft.Kind() != reflect.Func {
		return nil, errors.New("kruda: factory must be a function")
	}
	if ft.NumIn() != 0 {
		return nil, errors.New("kruda: factory must take no arguments")
	}
	if ft.NumOut() < 1 {
		return nil, errors.New("kruda: factory must return at least one value")
	}
	if ft.NumOut() >= 2 && !ft.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, errors.New("kruda: factory second return value must be error")
	}
	return ft.Out(0), nil
}

// GiveTransient registers a factory that creates a new instance on every Use[T]() call.
// factory must be a function with signature func() (T, error) or func() T.
func (c *Container) GiveTransient(factory any) error {
	returnType, err := validateFactory(factory)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isRegistered(returnType) {
		return fmt.Errorf("kruda: duplicate registration for type %v", returnType)
	}
	c.transients[returnType] = &transientEntry{
		factory: wrapFactory(factory),
	}
	return nil
}

// GiveLazy registers a factory that runs once on first Use[T]() call.
// Subsequent calls return the cached result. If the factory returns an error,
// the result is not cached and the factory will be retried on the next call.
func (c *Container) GiveLazy(factory any) error {
	returnType, err := validateFactory(factory)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isRegistered(returnType) {
		return fmt.Errorf("kruda: duplicate registration for type %v", returnType)
	}
	c.lazies[returnType] = &lazyEntry{
		factory: wrapFactory(factory),
	}
	return nil
}

// GiveNamed registers a named singleton instance.
// Multiple instances of the same type can be registered with different names.
// The instance participates in lifecycle (OnInit/OnShutdown) and health checks.
func (c *Container) GiveNamed(name string, instance any) error {
	if instance == nil {
		return errors.New("kruda: cannot register nil value")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.named[name]; exists {
		return fmt.Errorf("kruda: duplicate named registration %q", name)
	}
	c.named[name] = instance
	c.initOrder = append(c.initOrder, instance)
	return nil
}

// wrapFactory wraps a typed factory func() (T, error) or func() T into func() (any, error)
// using reflection. Called once at registration time.
func wrapFactory(factory any) func() (any, error) {
	fv := reflect.ValueOf(factory)
	ft := fv.Type()
	return func() (any, error) {
		results := fv.Call(nil)
		inst := results[0].Interface()
		if ft.NumOut() == 2 && !results[1].IsNil() {
			return nil, results[1].Interface().(error)
		}
		return inst, nil
	}
}

// isRegistered checks if a type is already registered in any of the typed maps.
// Must be called with c.mu held (read or write).
func (c *Container) isRegistered(t reflect.Type) bool {
	if _, ok := c.singletons[t]; ok {
		return true
	}
	if _, ok := c.lazies[t]; ok {
		return true
	}
	if _, ok := c.transients[t]; ok {
		return true
	}
	return false
}

// goid returns the current goroutine ID by parsing runtime.Stack() output.
// The format "goroutine NNN [" has been stable since Go 1.0.
// Used only for circular dependency detection — not on the hot path.
// Returns 0 if parsing fails (cycle detection degrades to no-op, no false positives).
func goid() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Skip "goroutine " prefix (10 bytes)
	if n <= 10 {
		return 0
	}
	s := buf[10:n]
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			id, _ := strconv.ParseInt(string(s[:i]), 10, 64)
			return id
		}
	}
	return 0
}

// pushResolving pushes a type onto the current goroutine's resolution stack.
// Returns an error if the type is already on the stack (circular dependency).
func (c *Container) pushResolving(t reflect.Type) error {
	id := goid()
	val, _ := c.resolving.LoadOrStore(id, &[]reflect.Type{})
	stack := val.(*[]reflect.Type)

	// Check for cycle
	for _, s := range *stack {
		if s == t {
			// Build chain: existing stack + the repeated type
			names := make([]string, len(*stack)+1)
			for i, st := range *stack {
				names[i] = st.String()
			}
			names[len(*stack)] = t.String()
			return fmt.Errorf("kruda: circular dependency: %s", strings.Join(names, " → "))
		}
	}

	*stack = append(*stack, t)
	return nil
}

// popResolving removes the last type from the current goroutine's resolution stack.
// If the stack becomes empty, the entry is deleted from the map.
func (c *Container) popResolving() {
	id := goid()
	val, ok := c.resolving.Load(id)
	if !ok {
		return
	}
	stack := val.(*[]reflect.Type)
	if len(*stack) > 0 {
		*stack = (*stack)[:len(*stack)-1]
	}
	if len(*stack) == 0 {
		c.resolving.Delete(id)
	}
}

// Use resolves a service of type T from the container.
// Checks singletons first (no cycle detection), then transients, then lazy singletons.
// Returns (zero, error) if the type is not registered or a factory fails.
func Use[T any](c *Container) (T, error) {
	var zero T
	t := reflect.TypeOf((*T)(nil)).Elem()

	c.mu.RLock()
	// Singletons skip cycle detection — already constructed
	if inst, ok := c.singletons[t]; ok {
		c.mu.RUnlock()
		typed, ok := inst.(T)
		if !ok {
			return zero, fmt.Errorf("kruda: type mismatch for %v: stored %T", t, inst)
		}
		return typed, nil
	}
	if entry, ok := c.transients[t]; ok {
		c.mu.RUnlock()
		if err := c.pushResolving(t); err != nil {
			return zero, err
		}
		defer c.popResolving()
		inst, err := entry.factory()
		if err != nil {
			return zero, fmt.Errorf("kruda: transient factory for %v failed: %w", t, err)
		}
		typed, ok := inst.(T)
		if !ok {
			return zero, fmt.Errorf("kruda: type mismatch for %v: factory returned %T", t, inst)
		}
		return typed, nil
	}
	if entry, ok := c.lazies[t]; ok {
		c.mu.RUnlock()
		if entry.done.Load() {
			typed, ok := entry.instance.(T)
			if !ok {
				return zero, fmt.Errorf("kruda: type mismatch for %v: stored %T", t, entry.instance)
			}
			return typed, nil
		}
		if err := c.pushResolving(t); err != nil {
			return zero, err
		}
		defer c.popResolving()
		inst, first, err := entry.resolve()
		if err != nil {
			return zero, fmt.Errorf("kruda: lazy factory for %v failed: %w", t, err)
		}
		if first {
			c.mu.Lock()
			c.initOrder = append(c.initOrder, inst)
			c.mu.Unlock()
		}
		typed, ok := inst.(T)
		if !ok {
			return zero, fmt.Errorf("kruda: type mismatch for %v: factory returned %T", t, inst)
		}
		return typed, nil
	}
	c.mu.RUnlock()
	return zero, fmt.Errorf("kruda: no provider for type %v", t)
}

// UseNamed resolves a named instance of type T from the container.
// Returns error if the name is not found or the stored instance cannot be
// type-asserted to T.
func UseNamed[T any](c *Container, name string) (T, error) {
	var zero T
	c.mu.RLock()
	inst, ok := c.named[name]
	c.mu.RUnlock()
	if !ok {
		return zero, fmt.Errorf("kruda: no named instance %q", name)
	}
	typed, ok := inst.(T)
	if !ok {
		return zero, fmt.Errorf("kruda: named instance %q is %T, not %T", name, inst, zero)
	}
	return typed, nil
}

// MustUse is like Use but panics on error.
// Useful in setup/init code where a missing service is a programming error.
func MustUse[T any](c *Container) T {
	v, err := Use[T](c)
	if err != nil {
		panic(err)
	}
	return v
}

// MustUseNamed is like UseNamed but panics on error.
func MustUseNamed[T any](c *Container, name string) T {
	v, err := UseNamed[T](c, name)
	if err != nil {
		panic(err)
	}
	return v
}

// resolve attempts to initialize the lazy singleton.
// Uses sync.Mutex (not sync.Once) so that if the factory returns an error,
// done remains false and the factory will be retried on the next call.
// Returns (instance, firstResolve, error).
func (le *lazyEntry) resolve() (any, bool, error) {
	le.mu.Lock()
	defer le.mu.Unlock()
	if le.done.Load() {
		return le.instance, false, nil
	}
	inst, err := le.factory()
	if err != nil {
		return nil, false, err // don't set done — allow retry
	}
	le.instance = inst
	le.done.Store(true)
	return inst, true, nil
}

// Initializer is implemented by services that need startup initialization.
// Services implementing this interface will have OnInit called during
// container.Start() in registration order.
type Initializer interface {
	OnInit(ctx context.Context) error
}

// Shutdowner is implemented by services that need cleanup on shutdown.
// Services implementing this interface will have OnShutdown called during
// container.Shutdown() in reverse registration order.
type Shutdowner interface {
	OnShutdown(ctx context.Context) error
}

// Start calls OnInit on all services in initOrder that implement Initializer,
// in registration/resolution order. If any OnInit fails, already-initialized
// services that implement Shutdowner get OnShutdown called for cleanup in reverse order.
func (c *Container) Start(ctx context.Context) error {
	c.mu.RLock()
	order := make([]any, len(c.initOrder))
	copy(order, c.initOrder)
	c.mu.RUnlock()

	var initialized []Shutdowner
	for _, inst := range order {
		if init, ok := inst.(Initializer); ok {
			if err := init.OnInit(ctx); err != nil {
				for i := len(initialized) - 1; i >= 0; i-- {
					_ = initialized[i].OnShutdown(ctx)
				}
				return fmt.Errorf("kruda: OnInit failed for %T: %w", inst, err)
			}
		}
		if sd, ok := inst.(Shutdowner); ok {
			initialized = append(initialized, sd)
		}
	}
	return nil
}

// Shutdown calls OnShutdown on all services in initOrder that implement
// Shutdowner, in reverse registration/resolution order. All errors are
// collected and returned via errors.Join.
func (c *Container) Shutdown(ctx context.Context) error {
	c.mu.RLock()
	order := make([]any, len(c.initOrder))
	copy(order, c.initOrder)
	c.mu.RUnlock()

	var errs []error
	for i := len(order) - 1; i >= 0; i-- {
		if sd, ok := order[i].(Shutdowner); ok {
			if err := sd.OnShutdown(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// InjectMiddleware returns a middleware that makes the container available
// in the request context via c.Set("container", container).
// Use with app.Use(container.InjectMiddleware()) to enable Resolve[T]()
// lookups from the context locals (in addition to app-level fallback).
func (c *Container) InjectMiddleware() HandlerFunc {
	return func(ctx *Ctx) error {
		ctx.Set("container", c)
		return ctx.Next()
	}
}

// Module is the interface for modular DI registration.
// Modules group related service registrations into reusable units.
type Module interface {
	Install(c *Container) error
}

// Module installs a DI module into the App.
// If no container is configured, a new one is created automatically.
// Panics if Install returns an error — module registration failures are
// considered programming errors. Returns the App for method chaining.
func (app *App) Module(m Module) *App {
	if app.container == nil {
		app.container = NewContainer()
	}
	if err := m.Install(app.container); err != nil {
		panic(fmt.Sprintf("kruda: module install failed: %v", err))
	}
	return app
}

// resolveContainer returns the DI container from the request context.
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
func Resolve[T any](c *Ctx) (T, error) {
	var zero T
	container, err := resolveContainer(c)
	if err != nil {
		return zero, err
	}
	return Use[T](container)
}

// ResolveNamed resolves a named instance of type T from the DI container.
func ResolveNamed[T any](c *Ctx, name string) (T, error) {
	var zero T
	container, err := resolveContainer(c)
	if err != nil {
		return zero, err
	}
	return UseNamed[T](container, name)
}

// MustResolve is like Resolve but panics on error.
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
