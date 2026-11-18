package kruda

import (
	"strings"
	"testing"
)

func TestRouteWithTooManyParamsPanicsAtRegistration(t *testing.T) {
	ok := "/:a/:b/:c/:d/:e/:f/:g/*h"
	newRouter().addRoute("GET", ok, []HandlerFunc{func(*Ctx) error { return nil }})

	tooMany := "/:a/:b/:c/:d/:e/:f/:g<\\d+>/:h?/:i"
	defer func() {
		r := recover()
		msg, _ := r.(string)
		if !strings.Contains(msg, tooMany) || !strings.Contains(msg, "9 path parameters") {
			t.Fatalf("recover() = %v, want a panic naming the route and 9 path parameters", r)
		}
	}()
	newRouter().addRoute("GET", tooMany, []HandlerFunc{func(*Ctx) error { return nil }})
}
