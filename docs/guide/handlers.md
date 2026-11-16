# Handlers

## Basic Handlers

A handler receives a `*Ctx` and returns an error:

```go
app.Get("/hello", func(c *kruda.Ctx) error {
    return c.Text("Hello, World!")
})
```

The `HandlerFunc` type:

```go
type HandlerFunc func(c *Ctx) error
```

## Typed Handlers with C[T]

Typed handlers use Go generics to auto-parse request data into a struct:

```go
type CreateUserInput struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"min=0,max=150"`
}

type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

kruda.Post[CreateUserInput, User](app, "/users", func(c *kruda.C[CreateUserInput]) (*User, error) {
    return &User{ID: "1", Name: c.In.Name, Email: c.In.Email}, nil
})
```

The `C[T]` type extends `Ctx` with a typed `In` field:

```go
type C[T any] struct {
    *Ctx
    In T // parsed input from request
}
```

## Typed Handler Registration

Package-level generic functions register typed handlers:

```go
kruda.Get[In, Out](app, path, handler, opts...)
kruda.Post[In, Out](app, path, handler, opts...)
kruda.Put[In, Out](app, path, handler, opts...)
kruda.Delete[In, Out](app, path, handler, opts...)
kruda.Patch[In, Out](app, path, handler, opts...)
```

Short variants (no error return, for prototyping):

```go
kruda.GetX[In, Out](app, path, handler, opts...)
kruda.PostX[In, Out](app, path, handler, opts...)
```

Group-level typed handlers:

```go
kruda.GroupGet[In, Out](g, path, handler, opts...)
kruda.GroupPost[In, Out](g, path, handler, opts...)
```

## Struct Tags

Control how request data is bound to your struct:

```go
type GetUserInput struct {
    ID     string `param:"id"`              // from route parameter :id
    Fields string `query:"fields"`          // from query string ?fields=name,email
}

type UpdateUserInput struct {
    ID   string `param:"id"`               // from route parameter
    Name string `json:"name"`              // from JSON body
    Age  int    `json:"age"`               // from JSON body
}
```

Supported tag types:

| Tag | Source | Example |
|-----|--------|---------|
| `param` | Route parameters (`:id`) | `param:"id"` |
| `query` | Query string (`?key=val`) | `query:"fields"` |
| `json` | JSON request body | `json:"name"` |
| `header` | Request headers | `header:"X-Request-ID"` |

## Validation

After binding, `validate` struct tags are checked. Use them like this:

```go
type Input struct {
    Name  string `json:"name"  validate:"required,min=2,max=100"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"min=0,max=150"`
}
```


::: tip Validation runs by default
Since v1.7.1 `validate` tags are enforced without any configuration. Opt out with
`kruda.New(kruda.WithoutValidation())`, which parses input and checks nothing.
`kruda.WithValidator` is for registering custom rules or messages, not for
switching validation on.

Kruda implements 20 rules — `(*kruda.Validator).RuleNames()` lists them, plus any registered with `Register`.
It also supports the two `go-playground/validator` modifiers that change how the
rules around them apply: `omitempty` skips a field's rules when its value is the
zero value, and `dive` applies every rule after it to each element of a slice,
array or map rather than to the container. A `dive` failure names the element —
`tags[2]`, `limits[free]` — not just the field.

```go
Bio  string   `validate:"omitempty,min=10"`  // optional, but ≥10 chars if present
IDs  []string `validate:"omitempty,dive,uuid"` // every element must be a UUID
```

The tag syntax resembles `go-playground/validator`, which has far more rules, so a
tag carried over from it may name one that does not exist here. Those are skipped
with a startup warning naming the rule; the field's other rules still apply. The
generated OpenAPI schema advertises the constraints regardless.
:::

When validation fails, Kruda returns a structured error response:

```json
{
  "error": "Validation failed",
  "code": 422,
  "details": [
    { "field": "email", "message": "must be a valid email" }
  ]
}
```

## Response Methods

Handlers use `Ctx` methods to send responses:

```go
// JSON response (status set via Status())
c.Status(200).JSON(user)
c.JSON(user) // default status 200

// Text response
c.Status(200).Text("OK")
c.Text("OK")

// HTML response
c.HTML("<h1>Hello</h1>")

// No content (204)
c.NoContent()

// With headers (method chaining)
c.Status(200).SetHeader("X-Custom", "value").JSON(data)

// Redirect (default 302)
c.Redirect("/new-location")
c.Redirect("/new-location", 301)
```

See [Context API](/api/context) for all response methods.

## Route Options

Add metadata for OpenAPI generation:

```go
kruda.Post[CreateUserInput, User](app, "/users", handler,
    kruda.WithDescription("Create a new user"),
    kruda.WithTags("users"),
)
```

## Generated Binding (experimental)

For hot typed routes with many query fields, the generator pre-builds the
binding loop and the `min`/`max` checks, so per-request reflection is gone
from both paths. The output is ordinary Go that calls only the public API.

The CLI runs the generator from the Kruda core version selected by your
project's module graph. It requires Go and the `kruda` CLI on `PATH`, and a
core version that contains `cmd/bindgen`.

1. Annotate the input and add a `go:generate` line:

```go
//go:generate kruda generate binder --input input.go --type SearchInput --output search_generated.go --func bindSearchInput

type SearchInput struct {
    Q    string `query:"q" default:"all" validate:"min=1,max=64"`
    Page int    `query:"page" default:"1" validate:"min=1,max=1000"`
}
```

2. Run `go generate ./...`, or invoke the CLI directly:

```sh
kruda generate binder --input input.go --type SearchInput --output search_generated.go --func bindSearchInput
```

All four flags are required. Paths are relative to the working directory,
and the output directory must already exist. Successful generation replaces
the requested output file. Go may download dependencies or update module
bookkeeping under your project's normal module settings.

The standalone command remains supported, including in `go:generate` lines:

```sh
go run github.com/go-kruda/kruda/cmd/bindgen -input input.go -type SearchInput -output search_generated.go -func bindSearchInput
```

3. Pass the emitted plan to the route manually:

```go
kruda.Get[SearchInput, SearchOutput](app, "/search", handler,
    kruda.WithGeneratedPlan(SearchInputPlan()),
)
```

To generate registration helpers that select the plan automatically, add
`--routes` to the same command:

```sh
kruda generate binder --input input.go --type SearchInput --output search_generated.go --func bindSearchInput --routes
```

The standalone equivalent uses `-routes`:

```sh
go run github.com/go-kruda/kruda/cmd/bindgen -input input.go -type SearchInput -output search_generated.go -func bindSearchInput -routes
```

Keep the generated file beside `SearchInput` in the same package. Register app
or group routes with an ordinary typed callback. Go infers the output type:

```go
type SearchOutput struct {
    Q    string
    Page int
}

func search(c *kruda.C[SearchInput]) (*SearchOutput, error) {
    return &SearchOutput{Q: c.In.Q, Page: c.In.Page}, nil
}

func register(app *kruda.App) {
    SearchInputGet(app, "/search", search,
        kruda.WithDescription("Search the catalog"))
    api := app.Group("/api/v1")
    SearchInputGroupPost(api, "/search", search)
}
```

For input `SearchInput`, the generated app helpers are `SearchInputGet`,
`SearchInputPost`, `SearchInputPut`, `SearchInputDelete`, and `SearchInputPatch`.
The group helpers are `SearchInputGroupGet`, `SearchInputGroupPost`,
`SearchInputGroupPut`, `SearchInputGroupDelete`, and `SearchInputGroupPatch`.
Group helpers preserve prefixes and inherited middleware. There are no generated
HEAD, OPTIONS, arbitrary-method, or X/no-error callback helpers. POST and the
other methods retain the same query/path binding scope below.

Helper names are the input type name followed by the method, so two input
types in one package where one name is the other plus `Group`, such as
`Search` and `SearchGroup`, both produce `SearchGroupGet` and the package does
not compile. Rename one of the types, or generate their routes in separate
packages.

Each helper prepends `WithGeneratedPlan(SearchInputPlan())` before caller options
in a fresh slice. Caller option order and backing arrays are preserved. The last
plan for the same input type wins, including an empty or stale caller plan that
causes generic parsing. A plan for another input type leaves the generated
default selected. Metadata follows the existing typed route option behavior.
Typed registrars currently do not forward Wing presets to the transport;
these helpers retain that limitation.

Route helper generation is off by default. Omitting the switch or using
`--routes=false` retains binder-only output. `--routes=true` also enables it.
Standalone bindgen accepts the corresponding `-routes=false` and `-routes=true`.
The module-selected core generator must support `-routes`; updating the installed
CLI alone does not add that support. Repeating generation with the same input
and flags produces the same bytes. Invalid flags, unsupported inputs, and
collisions among the requested input and emitted declarations fail before
replacing output. With routes enabled, the binder name and package declarations
must not shadow `string`, `make`, `len`, or `append`, which the helpers use.
The generator checks the input and same-package sibling Go files, including
files for other build configurations. Receiver methods and local bindings are
allowed. Other package declaration conflicts remain Go compiler errors.
Output directories must already exist, and write errors are returned.

Scope: `query`/`param` scalar fields (string, bool, ints, uints, floats)
with optional `default` tags, plus built-in `min`/`max` validation.
`json`/`form` body fields are rejected at generation time.

Everything fails closed. The plan carries the shape the code was generated
for; if the struct drifts (added, removed, renamed, or retagged field),
registration drops the stale binder silently and the route keeps the generic
parser. Inputs whose rules the generator cannot compile — custom overrides,
`dive`, `omitempty`, anything outside `min`/`max` — decline the compiled
validator and keep generic validation. Regenerate after changing the input
to stay on the fast path.

The same checks apply through generated registration helpers. A validate-only
tag change can retain generated binding while validation falls back separately.
Validation tags stay inactive with `kruda.WithValidator(nil)`. Custom validation
rules must be configured before registration. Runtime fallback requires the
stale generated Go file to compile; removing or retyping fields can require
regeneration before the application builds.
