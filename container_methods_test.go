package kruda

import (
	"errors"
	"strings"
	"testing"
)

// --- MustUse panic paths ---

func TestContainer_MustUse_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustUse should panic when service not registered")
		}
	}()

	c := NewContainer()
	MustUse[string](c)
}

func TestContainer_MustUseNamed_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustUseNamed should panic when service not registered")
		}
	}()

	c := NewContainer()
	MustUseNamed[string](c, "missing")
}

// --- validateFactory ---

func TestValidateFactory_NilInput(t *testing.T) {
	_, err := validateFactory(nil)
	if err == nil {
		t.Error("expected error for nil factory")
	}
}

func TestValidateFactory_NonFunction(t *testing.T) {
	_, err := validateFactory("not a func")
	if err == nil {
		t.Error("expected error for non-function")
	}
}

func TestValidateFactory_FunctionWithArgs(t *testing.T) {
	_, err := validateFactory(func(x int) int { return x })
	if err == nil {
		t.Error("expected error for function with args")
	}
}

func TestValidateFactory_FunctionNoReturn(t *testing.T) {
	_, err := validateFactory(func() {})
	if err == nil {
		t.Error("expected error for function with no return")
	}
}

func TestValidateFactory_SecondReturnNotError(t *testing.T) {
	_, err := validateFactory(func() (int, string) { return 0, "" })
	if err == nil {
		t.Error("expected error when second return is not error")
	}
}

func TestValidateFactory_ValidSingleReturn(t *testing.T) {
	rt, err := validateFactory(func() int { return 42 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.String() != "int" {
		t.Errorf("return type = %v, want int", rt)
	}
}

func TestValidateFactory_ValidWithError(t *testing.T) {
	rt, err := validateFactory(func() (string, error) { return "", nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.String() != "string" {
		t.Errorf("return type = %v, want string", rt)
	}
}

// --- GiveTransient / GiveLazy registration errors ---

func TestGiveTransient_InvalidFactory(t *testing.T) {
	c := NewContainer()
	err := c.GiveTransient("not a func")
	if err == nil {
		t.Error("expected error for invalid factory")
	}
}

func TestGiveTransient_Duplicate(t *testing.T) {
	c := NewContainer()
	err := c.GiveTransient(func() int { return 1 })
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	err = c.GiveTransient(func() int { return 2 })
	if err == nil {
		t.Error("expected duplicate registration error")
	}
}

func TestGiveLazy_InvalidFactory(t *testing.T) {
	c := NewContainer()
	err := c.GiveLazy("not a func")
	if err == nil {
		t.Error("expected error for invalid factory")
	}
}

func TestGiveLazy_Duplicate(t *testing.T) {
	c := NewContainer()
	err := c.GiveLazy(func() int { return 1 })
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	err = c.GiveLazy(func() int { return 2 })
	if err == nil {
		t.Error("expected duplicate registration error")
	}
}

// --- MustUse / MustUseNamed success ---

func TestMustUse_Success(t *testing.T) {
	c := NewContainer()
	_ = c.Give("hello")
	got := MustUse[string](c)
	if got != "hello" {
		t.Errorf("MustUse = %q", got)
	}
}

func TestMustUseNamed_Success(t *testing.T) {
	c := NewContainer()
	_ = c.GiveNamed("greeting", "hi")
	got := MustUseNamed[string](c, "greeting")
	if got != "hi" {
		t.Errorf("MustUseNamed = %q", got)
	}
}

// --- ResolveNamed / MustResolveNamed via handler ---

func TestResolveNamed_ViaHandler(t *testing.T) {
	c := NewContainer()
	_ = c.GiveNamed("db", "postgres://localhost")

	app := New(WithContainer(c))
	app.Get("/test", func(ctx *Ctx) error {
		val, err := ResolveNamed[string](ctx, "db")
		if err != nil {
			return BadRequest(err.Error())
		}
		return ctx.Text(val)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "postgres://localhost") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestResolveNamed_NoContainer(t *testing.T) {
	app := New()
	app.Get("/test", func(ctx *Ctx) error {
		_, err := ResolveNamed[string](ctx, "db")
		if err == nil {
			return BadRequest("expected error")
		}
		return ctx.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestMustResolveNamed_ViaHandler(t *testing.T) {
	c := NewContainer()
	_ = c.GiveNamed("msg", "hello world")

	app := New(WithContainer(c))
	app.Get("/test", func(ctx *Ctx) error {
		val := MustResolveNamed[string](ctx, "msg")
		return ctx.Text(val)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "hello world") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestMustResolveNamed_Panics(t *testing.T) {
	c := NewContainer()
	app := New(WithContainer(c))
	app.Get("/test", func(ctx *Ctx) error {
		defer func() {
			recover() // catch panic
		}()
		MustResolveNamed[string](ctx, "nonexistent")
		return ctx.Text("should not reach")
	})
	app.Compile()

	tc := NewTestClient(app)
	// The panic should be caught internally by defer/recover
	tc.Get("/test")
}

// --- isRegistered branch coverage ---

func TestIsRegistered_TransientBranch(t *testing.T) {
	c := NewContainer()
	_ = c.GiveTransient(func() (int, error) { return 1, nil })
	// Now trying to register a singleton of the same type should fail
	err := c.Give(42)
	if err == nil {
		t.Error("expected duplicate registration error for int (transient already registered)")
	}
}

func TestIsRegistered_LazyBranch(t *testing.T) {
	c := NewContainer()
	_ = c.GiveLazy(func() (int, error) { return 1, nil })
	// Now trying to register a transient of the same type should fail
	err := c.GiveTransient(func() (int, error) { return 2, nil })
	if err == nil {
		t.Error("expected duplicate registration error for int (lazy already registered)")
	}
}

// --- GiveLazy retry on error ---

func TestGiveLazy_RetryOnError(t *testing.T) {
	c := NewContainer()
	callCount := 0
	_ = c.GiveLazy(func() (string, error) {
		callCount++
		if callCount == 1 {
			return "", errors.New("temporary failure")
		}
		return "success", nil
	})

	// First call should fail
	_, err := Use[string](c)
	if err == nil {
		t.Error("expected error on first call")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}

	// Second call should succeed (retry)
	val, err := Use[string](c)
	if err != nil {
		t.Fatalf("expected success on retry, got: %v", err)
	}
	if val != "success" {
		t.Errorf("val = %q", val)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

// --- GiveAs error paths ---

func TestGiveAs_NilInstance(t *testing.T) {
	c := NewContainer()
	err := c.GiveAs(nil, (*testInterface)(nil))
	if err == nil {
		t.Error("expected error for nil instance")
	}
}

func TestGiveAs_NilIfacePtr(t *testing.T) {
	c := NewContainer()
	err := c.GiveAs(&testImpl{}, nil)
	if err == nil {
		t.Error("expected error for nil ifacePtr")
	}
}

func TestGiveAs_NonPointerToInterface(t *testing.T) {
	c := NewContainer()
	err := c.GiveAs(&testImpl{}, "not a pointer")
	if err == nil {
		t.Error("expected error for non-pointer ifacePtr")
	}
}

func TestGiveAs_NotImplementing(t *testing.T) {
	c := NewContainer()
	type notImpl struct{}
	err := c.GiveAs(&notImpl{}, (*testInterface)(nil))
	if err == nil {
		t.Error("expected error when instance doesn't implement interface")
	}
}

func TestGiveAs_Duplicate(t *testing.T) {
	c := NewContainer()
	_ = c.GiveAs(&testImpl{msg: "a"}, (*testInterface)(nil))
	err := c.GiveAs(&testImpl{msg: "b"}, (*testInterface)(nil))
	if err == nil {
		t.Error("expected duplicate registration error")
	}
}

// --- Transient factory error path ---

func TestGiveTransient_FactoryError(t *testing.T) {
	c := NewContainer()
	_ = c.GiveTransient(func() (string, error) {
		return "", errors.New("factory failed")
	})
	_, err := Use[string](c)
	if err == nil {
		t.Error("expected error from failed transient factory")
	}
}

// --- MustResolve success ---

func TestMustResolve_Success(t *testing.T) {
	c := NewContainer()
	_ = c.Give("hello")

	app := New(WithContainer(c))
	app.Get("/test", func(ctx *Ctx) error {
		val := MustResolve[string](ctx)
		return ctx.Text(val)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if !strings.Contains(resp.BodyString(), "hello") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- WithResourceIDParam (resource config option) ---

func TestWithResourceIDParam(t *testing.T) {
	cfg := defaultResourceConfig()
	WithResourceIDParam("uuid")(&cfg)
	if cfg.idParam != "uuid" {
		t.Errorf("idParam = %q", cfg.idParam)
	}
}
