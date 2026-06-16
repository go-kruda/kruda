package kruda

import "testing"

type typedClientCreateUser struct {
	Name string `json:"name"`
}

type typedClientUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestTypedTestClientPostTyped(t *testing.T) {
	app := New()
	Post[typedClientCreateUser, typedClientUser](app, "/users", func(c *C[typedClientCreateUser]) (*typedClientUser, error) {
		return &typedClientUser{ID: "u1", Name: c.In.Name}, nil
	})
	app.Compile()

	resp, err := PostTyped[typedClientCreateUser, typedClientUser](
		NewTestClient(app),
		"/users",
		typedClientCreateUser{Name: "Ada"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode())
	}
	if resp.Body.ID != "u1" || resp.Body.Name != "Ada" {
		t.Fatalf("body = %+v, want u1/Ada", resp.Body)
	}
}

func TestSendTypedUsesRequestBuilder(t *testing.T) {
	type queryIn struct {
		Name string `query:"name"`
	}

	app := New()
	Get[queryIn, typedClientUser](app, "/users", func(c *C[queryIn]) (*typedClientUser, error) {
		return &typedClientUser{ID: c.Header("X-User-ID"), Name: c.In.Name}, nil
	})
	app.Compile()

	resp, err := SendTyped[typedClientUser](
		NewTestClient(app).Request("GET", "/users").
			Header("X-User-ID", "u2").
			Query("name", "Grace"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Body.ID != "u2" || resp.Body.Name != "Grace" {
		t.Fatalf("body = %+v, want u2/Grace", resp.Body)
	}
}

func TestTypedTestClientNoContent(t *testing.T) {
	app := New()
	Delete[struct {
		ID string `param:"id"`
	}, typedClientUser](app, "/users/:id", func(c *C[struct {
		ID string `param:"id"`
	}]) (*typedClientUser, error) {
		return nil, nil
	})
	app.Compile()

	resp, err := DeleteTyped[typedClientUser](NewTestClient(app), "/users/u1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode())
	}
	if resp.Body != (typedClientUser{}) {
		t.Fatalf("body = %+v, want zero value", resp.Body)
	}
}

func TestTypedHandlerProblemErrorIncludesProblemFields(t *testing.T) {
	type input struct {
		ID string `param:"id"`
	}

	app := New(WithProblemJSON())
	Get[input, typedClientUser](app, "/users/:id", func(c *C[input]) (*typedClientUser, error) {
		return nil, NotFound("user not found").
			WithType("https://errors.example.com/not-found").
			WithInstance("/problems/users/"+c.In.ID).
			With("userId", c.In.ID)
	})
	app.Compile()

	resp, err := GetTyped[map[string]any](NewTestClient(app), "/users/u1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode())
	}
	if ct := resp.Header("Content-Type"); ct != "application/problem+json; charset=utf-8" {
		t.Fatalf("content-type = %q, want application/problem+json; charset=utf-8", ct)
	}
	if resp.Body["type"] != "https://errors.example.com/not-found" {
		t.Fatalf("type = %v, want custom problem type", resp.Body["type"])
	}
	if resp.Body["instance"] != "/problems/users/u1" {
		t.Fatalf("instance = %v, want custom instance", resp.Body["instance"])
	}
	if resp.Body["userId"] != "u1" {
		t.Fatalf("userId = %v, want u1", resp.Body["userId"])
	}
}
