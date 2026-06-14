package kruda

import (
	"encoding/json"
	"testing"
)

func TestProblemDetailsMarshal_full(t *testing.T) {
	p := ProblemDetails{
		Type: "https://errors.example.com/not-found", Title: "Not Found", Status: 404,
		Detail: "user not found", Instance: "/users/42",
		Extensions: map[string]any{"userId": "42"},
	}
	got, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	// encoding/json sorts map keys → deterministic alphabetical order.
	want := `{"detail":"user not found","instance":"/users/42","status":404,"title":"Not Found","type":"https://errors.example.com/not-found","userId":"42"}`
	if string(got) != want {
		t.Fatalf("\n got: %s\nwant: %s", got, want)
	}
}

func TestProblemDetailsMarshal_defaultsAndErrors(t *testing.T) {
	p := ProblemDetails{
		Title: "Validation failed", Status: 422, Detail: "email is required", Instance: "/users",
		Errors: []FieldError{{Field: "email", Rule: "required", Message: "email is required"}},
	}
	got, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"detail":"email is required","errors":[{"field":"email","rule":"required","param":"","message":"email is required","value":""}],"instance":"/users","status":422,"title":"Validation failed","type":"about:blank"}`
	if string(got) != want {
		t.Fatalf("\n got: %s\nwant: %s", got, want)
	}
}

func TestProblemDetailsMarshal_reservedExtensionIgnored(t *testing.T) {
	p := ProblemDetails{
		Title: "Bad Request", Status: 400,
		Extensions: map[string]any{"status": 999, "x": "ok"}, // "status" must not override
	}
	got, _ := json.Marshal(p)
	want := `{"status":400,"title":"Bad Request","type":"about:blank","x":"ok"}`
	if string(got) != want {
		t.Fatalf("\n got: %s\nwant: %s", got, want)
	}
}

func TestKrudaErrorBuilders(t *testing.T) {
	e := NotFound("user not found").
		WithType("https://errors.example.com/not-found").
		WithDetail("no such user").
		WithInstance("/users/42").
		With("userId", "42").
		With("type", "ignored-but-stored") // stored; renderer drops reserved keys

	if e.Code != 404 || e.Type != "https://errors.example.com/not-found" ||
		e.Detail != "no such user" || e.Instance != "/users/42" {
		t.Fatalf("builder fields wrong: %+v", e)
	}
	if e.Extensions["userId"] != "42" {
		t.Fatalf("With did not store extension: %+v", e.Extensions)
	}
}
