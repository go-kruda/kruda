// Cold-start subject: a Kruda server whose only interesting property is how
// many typed routes it registers.
//
// The CHANGELOG ties its cold-start figures to "25 typed POST routes with
// distinct 5-field schemas", the premise being that typed-handler registration
// is what drives Sonic's JIT warm-up. ROUTES makes that premise measurable
// rather than assumed.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/go-kruda/kruda"
)

// Each route needs its own struct type so the engine cannot reuse a compiled
// encoder between them. Generics are resolved at compile time, so this is a
// fixed set rather than a loop over a slice of types.
type (
	In00 struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
		City  string `json:"city"`
		Score int    `json:"score"`
	}
	Out struct {
		ID string `json:"id"`
	}
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3500"
	}
	n := 25
	if v := os.Getenv("ROUTES"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ROUTES:", err)
			os.Exit(2)
		}
		n = parsed
	}

	app := kruda.New()
	register(app, n)

	// Readiness probe. Registered last so a 200 here means every typed route
	// above is already compiled.
	app.Get("/ready", func(c *kruda.Ctx) error { return c.Text("ok") })

	if err := app.Listen(":" + port); err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
}

type In01 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In02 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In03 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In04 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In05 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In06 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In07 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In08 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In09 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In10 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In11 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In12 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In13 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In14 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In15 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In16 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In17 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In18 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In19 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In20 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In21 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In22 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In23 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In24 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In25 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In26 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In27 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In28 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In29 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In30 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

type In31 struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	City  string `json:"city"`
	Score int    `json:"score"`
}

// register wires the first n typed routes.
func register(app *kruda.App, n int) {
	if n > 0 {
		kruda.Post[In00, Out](app, "/r00", func(c *kruda.C[In00]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 1 {
		kruda.Post[In01, Out](app, "/r01", func(c *kruda.C[In01]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 2 {
		kruda.Post[In02, Out](app, "/r02", func(c *kruda.C[In02]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 3 {
		kruda.Post[In03, Out](app, "/r03", func(c *kruda.C[In03]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 4 {
		kruda.Post[In04, Out](app, "/r04", func(c *kruda.C[In04]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 5 {
		kruda.Post[In05, Out](app, "/r05", func(c *kruda.C[In05]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 6 {
		kruda.Post[In06, Out](app, "/r06", func(c *kruda.C[In06]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 7 {
		kruda.Post[In07, Out](app, "/r07", func(c *kruda.C[In07]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 8 {
		kruda.Post[In08, Out](app, "/r08", func(c *kruda.C[In08]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 9 {
		kruda.Post[In09, Out](app, "/r09", func(c *kruda.C[In09]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 10 {
		kruda.Post[In10, Out](app, "/r10", func(c *kruda.C[In10]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 11 {
		kruda.Post[In11, Out](app, "/r11", func(c *kruda.C[In11]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 12 {
		kruda.Post[In12, Out](app, "/r12", func(c *kruda.C[In12]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 13 {
		kruda.Post[In13, Out](app, "/r13", func(c *kruda.C[In13]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 14 {
		kruda.Post[In14, Out](app, "/r14", func(c *kruda.C[In14]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 15 {
		kruda.Post[In15, Out](app, "/r15", func(c *kruda.C[In15]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 16 {
		kruda.Post[In16, Out](app, "/r16", func(c *kruda.C[In16]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 17 {
		kruda.Post[In17, Out](app, "/r17", func(c *kruda.C[In17]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 18 {
		kruda.Post[In18, Out](app, "/r18", func(c *kruda.C[In18]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 19 {
		kruda.Post[In19, Out](app, "/r19", func(c *kruda.C[In19]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 20 {
		kruda.Post[In20, Out](app, "/r20", func(c *kruda.C[In20]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 21 {
		kruda.Post[In21, Out](app, "/r21", func(c *kruda.C[In21]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 22 {
		kruda.Post[In22, Out](app, "/r22", func(c *kruda.C[In22]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 23 {
		kruda.Post[In23, Out](app, "/r23", func(c *kruda.C[In23]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 24 {
		kruda.Post[In24, Out](app, "/r24", func(c *kruda.C[In24]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 25 {
		kruda.Post[In25, Out](app, "/r25", func(c *kruda.C[In25]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 26 {
		kruda.Post[In26, Out](app, "/r26", func(c *kruda.C[In26]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 27 {
		kruda.Post[In27, Out](app, "/r27", func(c *kruda.C[In27]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 28 {
		kruda.Post[In28, Out](app, "/r28", func(c *kruda.C[In28]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 29 {
		kruda.Post[In29, Out](app, "/r29", func(c *kruda.C[In29]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 30 {
		kruda.Post[In30, Out](app, "/r30", func(c *kruda.C[In30]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
	if n > 31 {
		kruda.Post[In31, Out](app, "/r31", func(c *kruda.C[In31]) (*Out, error) { return &Out{ID: c.In.Name}, nil })
	}
}
