package kruda

import (
	"testing"
)

// --- Validation built-in rules: gte/lte, gt/lt, contains, len ---

func TestValidation_GTE_LTE(t *testing.T) {
	type Input struct {
		Age int `query:"age" validate:"gte=18,lte=100"`
	}
	type Output struct {
		OK bool `json:"ok"`
	}

	app := New(WithValidator(NewValidator()))
	Get[Input, Output](app, "/check-age", func(c *C[Input]) (*Output, error) {
		return &Output{OK: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/check-age?age=25")
	if resp.StatusCode() != 200 {
		t.Errorf("valid age status = %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/check-age?age=10")
	if resp.StatusCode() != 422 {
		t.Errorf("too young status = %d, want 422", resp.StatusCode())
	}
}

func TestValidation_GT_LT(t *testing.T) {
	type Input struct {
		Score float64 `query:"score" validate:"gt=0,lt=100"`
	}
	type Output struct {
		OK bool `json:"ok"`
	}

	app := New(WithValidator(NewValidator()))
	Get[Input, Output](app, "/check-score", func(c *C[Input]) (*Output, error) {
		return &Output{OK: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/check-score?score=50")
	if resp.StatusCode() != 200 {
		t.Errorf("valid score status = %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/check-score?score=0")
	if resp.StatusCode() != 422 {
		t.Errorf("zero score status = %d, want 422", resp.StatusCode())
	}
}

func TestValidation_Contains(t *testing.T) {
	type Input struct {
		Name string `query:"name" validate:"contains=test"`
	}
	type Output struct {
		OK bool `json:"ok"`
	}

	app := New(WithValidator(NewValidator()))
	Get[Input, Output](app, "/check-name", func(c *C[Input]) (*Output, error) {
		return &Output{OK: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/check-name?name=my_test_val")
	if resp.StatusCode() != 200 {
		t.Errorf("contains status = %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/check-name?name=nope")
	if resp.StatusCode() != 422 {
		t.Errorf("not contains status = %d, want 422", resp.StatusCode())
	}
}

func TestValidation_Len(t *testing.T) {
	type Input struct {
		Code string `query:"code" validate:"len=6"`
	}
	type Output struct {
		OK bool `json:"ok"`
	}

	app := New(WithValidator(NewValidator()))
	Get[Input, Output](app, "/check-code", func(c *C[Input]) (*Output, error) {
		return &Output{OK: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/check-code?code=ABC123")
	if resp.StatusCode() != 200 {
		t.Errorf("valid len status = %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/check-code?code=AB")
	if resp.StatusCode() != 422 {
		t.Errorf("short len status = %d, want 422", resp.StatusCode())
	}
}

// --- App.Validator (eager + lazy init) ---

func TestApp_Validator(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	v := app.Validator()
	if v == nil {
		t.Error("Validator() returned nil")
	}
}

func TestApp_Validator_LazyInit(t *testing.T) {
	app := New() // no WithValidator
	v := app.Validator()
	if v == nil {
		t.Error("Validator() should lazy-init")
	}
	// Second call should return same instance
	v2 := app.Validator()
	if v != v2 {
		t.Error("Validator() should return same instance")
	}
}
