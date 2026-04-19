package kruda

import (
	"testing"
)

// --- Error constructors ---

func TestUnauthorized(t *testing.T) {
	e := Unauthorized("no auth")
	if e.Code != 401 || e.Message != "no auth" {
		t.Errorf("Unauthorized = %+v", e)
	}
}

func TestForbidden(t *testing.T) {
	e := Forbidden("denied")
	if e.Code != 403 || e.Message != "denied" {
		t.Errorf("Forbidden = %+v", e)
	}
}

func TestConflict(t *testing.T) {
	e := Conflict("dup")
	if e.Code != 409 || e.Message != "dup" {
		t.Errorf("Conflict = %+v", e)
	}
}

func TestUnprocessableEntity(t *testing.T) {
	e := UnprocessableEntity("bad data")
	if e.Code != 422 || e.Message != "bad data" {
		t.Errorf("UnprocessableEntity = %+v", e)
	}
}

func TestTooManyRequests(t *testing.T) {
	e := TooManyRequests("slow down")
	if e.Code != 429 || e.Message != "slow down" {
		t.Errorf("TooManyRequests = %+v", e)
	}
}

// --- MapErrorType ---

type customTestError struct {
	msg string
}

func (e *customTestError) Error() string { return e.msg }

func TestMapErrorType_Basic(t *testing.T) {
	app := New()
	MapErrorType[*customTestError](app, 422, "validation failed")

	app.Get("/fail", func(c *Ctx) error {
		return &customTestError{msg: "bad input"}
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 422 {
		t.Errorf("status = %d, want 422", resp.statusCode)
	}
}

func TestMapErrorType_PanicOnBareError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MapErrorType[error] should panic")
		}
	}()
	app := New()
	MapErrorType[error](app, 500, "catch all")
}
