package kruda

import (
	"errors"
	"testing"
	"testing/quick"
)

// Property: Provide/Need Round Trip
//
// For any key string and any value of type T, calling c.Provide(key, value)
// followed by Need[T](c, key) should return (value, true).

func TestPropertyProvideNeedRoundTrip(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	t.Run("String", func(t *testing.T) {
		f := func(key, value string) bool {
			if key == "" {
				return true
			}
			app := New()
			c := newCtx(app)
			c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

			c.Provide(key, value)
			got, ok := Need[string](c, key)
			return ok && got == value
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("Int", func(t *testing.T) {
		f := func(key string, value int) bool {
			if key == "" {
				return true
			}
			app := New()
			c := newCtx(app)
			c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

			c.Provide(key, value)
			got, ok := Need[int](c, key)
			return ok && got == value
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})
}

// Property: Need Returns False for Missing or Mistyped Keys
//
// For any Ctx and any key that was not set via Provide, Need[T](c, key) should
// return (zero, false). For any key set via Provide(key, valueOfTypeA), calling
// Need[B](c, key) where B is not assignable from A should return (zero, false).

func TestPropertyNeedMissingOrMistyped(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	t.Run("Missing", func(t *testing.T) {
		f := func(key string) bool {
			app := New()
			c := newCtx(app)
			c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

			val, ok := Need[string](c, key)
			return !ok && val == ""
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("Mistyped", func(t *testing.T) {
		f := func(key string, value int) bool {
			if key == "" {
				return true
			}
			app := New()
			c := newCtx(app)
			c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

			c.Provide(key, value)           // store int
			val, ok := Need[string](c, key) // ask for string
			return !ok && val == ""
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})
}

// Property: OnParse Hook Error Stops Pipeline
//
// For any set of OnParse hooks where hook N returns an error, hooks N+1, N+2, ...
// should not be called, and the user handler should not be called.

func TestPropertyOnParseErrorStopsPipeline(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	f := func(hookCount uint8) bool {
		n := int(hookCount%5) + 1 // 1-5 hooks
		errorAt := n / 2          // error at middle hook

		app := New()
		callCounts := make([]int, n)
		for i := 0; i < n; i++ {
			idx := i
			app.OnParse(func(c *Ctx, input any) error {
				callCounts[idx]++
				if idx == errorAt {
					return errors.New("hook error")
				}
				return nil
			})
		}

		handlerCalled := false
		h := buildTypedHandler[queryIn, userOut](app, "GET", "/test", func(c *C[queryIn]) (*userOut, error) {
			handlerCalled = true
			return &userOut{ID: "ok"}, nil
		}, nil)

		c := bindCtx("GET", "/test", nil, nil, nil)
		err := h(c)

		// Error should be returned
		if err == nil {
			return false
		}

		// Hooks before errorAt should be called once
		for i := 0; i <= errorAt; i++ {
			if callCounts[i] != 1 {
				return false
			}
		}
		// Hooks after errorAt should NOT be called
		for i := errorAt + 1; i < n; i++ {
			if callCounts[i] != 0 {
				return false
			}
		}
		// Handler should NOT be called
		return !handlerCalled
	}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// Property: OnParse Hook Mutation Visibility
//
// For any OnParse hook that modifies the input struct (via pointer), subsequent
// hooks and the validation step should see the modified values.

func TestPropertyOnParseMutationVisibility(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	type mutInput struct {
		Value int `query:"value" default:"0"`
	}

	f := func(newValue int) bool {
		app := New()

		// Hook that mutates the input
		app.OnParse(func(c *Ctx, input any) error {
			if req, ok := input.(*mutInput); ok {
				req.Value = newValue
			}
			return nil
		})

		// Second hook verifies mutation is visible
		var seenValue int
		app.OnParse(func(c *Ctx, input any) error {
			if req, ok := input.(*mutInput); ok {
				seenValue = req.Value
			}
			return nil
		})

		var handlerValue int
		h := buildTypedHandler[mutInput, userOut](app, "GET", "/test", func(c *C[mutInput]) (*userOut, error) {
			handlerValue = c.In.Value
			return &userOut{ID: "ok"}, nil
		}, nil)

		c := bindCtx("GET", "/test", nil, nil, nil)
		err := h(c)
		if err != nil {
			return false
		}

		// Both second hook and handler should see the mutated value
		return seenValue == newValue && handlerValue == newValue
	}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}
