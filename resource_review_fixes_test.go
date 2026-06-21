package kruda

import (
	"context"
	"encoding/json"
	"testing"
)

// =============================================================================
// Fix 2: named ID types — registration gate (Kind-based) vs resourceParseID
// (was concrete-type based) must be consistent. A named integer/string type
// must work end-to-end, not return a runtime 400 "invalid id".
// =============================================================================

// NamedID is a named integer type (Kind==Int64). It passes the registration
// gate (which switches on Kind) but historically failed resourceParseID (which
// switched on the concrete type and had no case for it).
type NamedID int64

type namedIDItem struct {
	ID   NamedID `json:"id"`
	Name string  `json:"name"`
}

type namedIDService struct{}

func (namedIDService) List(context.Context, int, int) ([]namedIDItem, int, error) {
	return nil, 0, nil
}
func (namedIDService) Create(_ context.Context, item namedIDItem) (namedIDItem, error) {
	return item, nil
}
func (namedIDService) Get(_ context.Context, id NamedID) (namedIDItem, error) {
	return namedIDItem{ID: id, Name: "ok"}, nil
}
func (namedIDService) Update(_ context.Context, id NamedID, item namedIDItem) (namedIDItem, error) {
	item.ID = id
	return item, nil
}
func (namedIDService) Delete(context.Context, NamedID) error { return nil }

func TestResource_NamedIntIDType_Get200(t *testing.T) {
	app := New()
	Resource[namedIDItem, NamedID](app, "/things", namedIDService{})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/things/5"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200 (named int ID must parse)\nbody: %s", resp.statusCode, resp.body)
	}
	var body namedIDItem
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if body.ID != 5 {
		t.Errorf("parsed id = %d, want 5", body.ID)
	}
}

// NamedStrID is a named string type (Kind==String).
type NamedStrID string

type namedStrIDItem struct {
	ID NamedStrID `json:"id"`
}

type namedStrIDService struct{}

func (namedStrIDService) List(context.Context, int, int) ([]namedStrIDItem, int, error) {
	return nil, 0, nil
}
func (namedStrIDService) Create(_ context.Context, item namedStrIDItem) (namedStrIDItem, error) {
	return item, nil
}
func (namedStrIDService) Get(_ context.Context, id NamedStrID) (namedStrIDItem, error) {
	return namedStrIDItem{ID: id}, nil
}
func (namedStrIDService) Update(_ context.Context, id NamedStrID, item namedStrIDItem) (namedStrIDItem, error) {
	item.ID = id
	return item, nil
}
func (namedStrIDService) Delete(context.Context, NamedStrID) error { return nil }

func TestResource_NamedStringIDType_Get200(t *testing.T) {
	app := New()
	Resource[namedStrIDItem, NamedStrID](app, "/things", namedStrIDService{})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/things/abc"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200 (named string ID must parse)\nbody: %s", resp.statusCode, resp.body)
	}
	var body namedStrIDItem
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if body.ID != "abc" {
		t.Errorf("parsed id = %q, want %q", body.ID, "abc")
	}
}

// A non-numeric path segment for a named integer ID must still 400.
func TestResource_NamedIntIDType_BadValue400(t *testing.T) {
	app := New()
	Resource[namedIDItem, NamedID](app, "/things", namedIDService{})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/things/notanint"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400 for non-numeric named-int id\nbody: %s", resp.statusCode, resp.body)
	}
}

// =============================================================================
// Fix 3: partial PUT omitting a required field → 422 (spec §8(b)). A PUT body
// that omits a `required` field must fail validation, reporting that field.
// =============================================================================

func TestResourceUpdate_PartialPUTOmitsRequired422(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	Resource(app, "/users", &validatedUserService{})
	app.Compile()

	// validatedUser requires "name" and "email"; this body omits "email".
	req := &mockRequest{
		method:  "PUT",
		path:    "/users/abc",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"x"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 422 {
		t.Fatalf("status = %d, want 422 (partial PUT omits required 'email')\nbody: %s", resp.statusCode, resp.body)
	}

	// Assert the ValidationError wire shape: {code, message, errors[]}.
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Errors  []struct {
			Field string `json:"field"`
			Rule  string `json:"rule"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if body.Code != 422 {
		t.Errorf("body.code = %d, want 422", body.Code)
	}
	if len(body.Errors) == 0 {
		t.Fatal("expected at least one field error")
	}
	// The omitted required field ("email") must be reported.
	var reportedEmail bool
	for _, fe := range body.Errors {
		if fe.Field == "email" && fe.Rule == "required" {
			reportedEmail = true
		}
	}
	if !reportedEmail {
		t.Errorf("omitted required field 'email' not reported as required; errors=%+v", body.Errors)
	}
}
