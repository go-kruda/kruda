package kruda

import (
	"fmt"
	"reflect"
	"testing"
)

func TestWithGeneratedPlanNonAddressableInput(t *testing.T) {
	type input struct{}
	app := New()
	binderCalls := 0
	Post[input, input](app, "/items", func(c *C[input]) (*input, error) {
		return &c.In, nil
	}, WithGeneratedPlan(GeneratedPlan[input]{
		Binder: func(*Ctx) (reflect.Value, error) {
			binderCalls++
			return reflect.ValueOf(input{}), nil
		},
	}))
	resp, err := NewTestClient(app).Post("/items", "{}")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if binderCalls != 1 {
		t.Fatalf("binder calls = %d, want 1", binderCalls)
	}
	if resp.StatusCode() != 200 || resp.BodyString() != "{}" {
		t.Fatalf("response = %d %q, want 200 {}", resp.StatusCode(), resp.BodyString())
	}
}

func TestTypedHandler_RetainedInputShallowCopy(t *testing.T) {
	type input struct {
		Name   string            `json:"name"`
		Values []string          `json:"values"`
		Labels map[string]string `json:"labels"`
	}
	app := New()
	var parsed, handled []*input
	app.OnParse(func(_ *Ctx, value any) error {
		in := value.(*input)
		in.Name = "parsed-" + in.Name
		parsed = append(parsed, in)
		return nil
	})
	Post[input, emptyOut](app, "/items", func(c *C[input]) (*emptyOut, error) {
		in := parsed[len(parsed)-1]
		if c.In.Name != in.Name {
			t.Fatalf("handler input name = %q, want hook name %q", c.In.Name, in.Name)
		}
		c.In.Name = "handled"
		c.In.Values[0] = "handled-value"
		c.In.Labels["source"] = "handled-label"
		if in.Name == "handled" || in.Values[0] != "handled-value" || in.Labels["source"] != "handled-label" {
			t.Fatalf("hook input did not preserve shallow copy semantics: %+v", in)
		}
		handled = append(handled, &c.In)
		return nil, nil
	})
	app.Compile()
	for _, name := range []string{"first", "second"} {
		resp := newMockResponse()
		body := []byte(fmt.Sprintf(`{"name":%q,"values":["original"],"labels":{"source":"original"}}`, name))
		app.ServeKruda(resp, &mockRequest{method: "POST", path: "/items", body: body})
		if resp.statusCode != 204 || len(resp.body) != 0 {
			t.Fatalf("response = %d %s, want empty 204", resp.statusCode, resp.body)
		}
	}
	if len(parsed) != 2 || len(handled) != 2 {
		t.Fatalf("retained inputs = %d parsed, %d handled, want two each", len(parsed), len(handled))
	}
	if parsed[0] == parsed[1] || handled[0] == handled[1] || parsed[0] == handled[0] || parsed[1] == handled[1] {
		t.Fatal("retained input pointers alias across copies or requests")
	}
	if parsed[0].Name != "parsed-first" || parsed[1].Name != "parsed-second" || handled[0].Name != "handled" || handled[1].Name != "handled" {
		t.Fatalf("retained value fields changed across requests: parsed=%+v,%+v handled=%+v,%+v", parsed[0], parsed[1], handled[0], handled[1])
	}
	parsed[0].Name = "hook-after-return"
	parsed[0].Values[0] = "hook-value"
	parsed[0].Labels["source"] = "hook-label"
	if handled[0].Name != "handled" || handled[0].Values[0] != "hook-value" || handled[0].Labels["source"] != "hook-label" {
		t.Fatalf("retained typed input lost shallow copy semantics: %+v", handled[0])
	}
	handled[0].Name = "handler-after-return"
	if parsed[0].Name != "hook-after-return" || parsed[1].Name != "parsed-second" || handled[1].Name != "handled" || handled[1].Values[0] != "handled-value" || handled[1].Labels["source"] != "handled-label" {
		t.Fatal("retained mutation changed an independent value or another request")
	}
}

func TestGeneratedBinderRetainedTypedInput(t *testing.T) {
	for _, generated := range []bool{false, true} {
		t.Run(fmt.Sprintf("generated=%t", generated), func(t *testing.T) {
			app := New()
			var parsed, handled []*generatedBinderInput
			app.OnParse(func(_ *Ctx, value any) error {
				in := value.(*generatedBinderInput)
				in.Name = "parsed-" + in.Name
				parsed = append(parsed, in)
				return nil
			})
			app.Get("/users/:id", generatedBinderParityHandler(app, generated, func(c *C[generatedBinderInput]) (*generatedBinderInput, error) {
				c.In.Name = "handled-" + c.In.Name
				handled = append(handled, &c.In)
				return nil, nil
			}))
			app.Compile()
			for _, name := range []string{"first", "second"} {
				resp := newMockResponse()
				app.ServeKruda(resp, &mockRequest{method: "GET", path: "/users/42", query: map[string]string{"name": name}})
				if resp.statusCode != 204 || len(resp.body) != 0 {
					t.Fatalf("response = %d %s, want empty 204", resp.statusCode, resp.body)
				}
			}
			if len(parsed) != 2 || len(handled) != 2 {
				t.Fatalf("retained inputs = %d parsed, %d handled, want two each", len(parsed), len(handled))
			}
			if parsed[0].Name != "parsed-first" || parsed[1].Name != "parsed-second" || handled[0].Name != "handled-parsed-first" || handled[1].Name != "handled-parsed-second" {
				t.Fatalf("retained inputs changed across requests: parsed=%+v,%+v handled=%+v,%+v", parsed[0], parsed[1], handled[0], handled[1])
			}
			parsed[0].Name = "hook-after-return"
			handled[1].Name = "handler-after-return"
			if handled[0].Name != "handled-parsed-first" || parsed[1].Name != "parsed-second" {
				t.Fatal("retained hook and typed input value fields alias")
			}
		})
	}
}
