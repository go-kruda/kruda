package kruda

// C is the generic typed context that embeds *Ctx with a parsed input field.
// Phase 1 stub: input is parsed from JSON body only.
// Phase 2 will add struct tag-based multi-source binding (param, query, body).
type C[T any] struct {
	*Ctx
	In T // parsed input from request body
}

// Get registers a typed GET handler. H5 fix: does NOT parse body for GET requests.
// Input struct is zero-valued. Phase 2 will bind from query/params via struct tags.
func Get[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error)) {
	app.Get(path, func(c *Ctx) error {
		tc := &C[In]{Ctx: c}
		result, err := handler(tc)
		if err != nil {
			return err
		}
		if result != nil {
			return c.JSON(result)
		}
		return c.NoContent()
	})
}

// Post registers a typed POST handler. It parses the request body as JSON
// into the In type, calls the handler, and serializes the result as JSON.
// Returns NoContent (204) when the handler returns a nil result.
func Post[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error)) {
	app.Post(path, func(c *Ctx) error {
		in, err := bindInput[In](c)
		if err != nil {
			return err
		}
		tc := &C[In]{Ctx: c, In: in}
		result, err := handler(tc)
		if err != nil {
			return err
		}
		if result != nil {
			return c.JSON(result)
		}
		return c.NoContent()
	})
}

// Put registers a typed PUT handler. It parses the request body as JSON
// into the In type, calls the handler, and serializes the result as JSON.
// Returns NoContent (204) when the handler returns a nil result.
func Put[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error)) {
	app.Put(path, func(c *Ctx) error {
		in, err := bindInput[In](c)
		if err != nil {
			return err
		}
		tc := &C[In]{Ctx: c, In: in}
		result, err := handler(tc)
		if err != nil {
			return err
		}
		if result != nil {
			return c.JSON(result)
		}
		return c.NoContent()
	})
}

// Delete registers a typed DELETE handler. H5 fix: does NOT parse body for DELETE.
// Input struct is zero-valued. Phase 2 will bind from query/params via struct tags.
func Delete[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error)) {
	app.Delete(path, func(c *Ctx) error {
		tc := &C[In]{Ctx: c}
		result, err := handler(tc)
		if err != nil {
			return err
		}
		if result != nil {
			return c.JSON(result)
		}
		return c.NoContent()
	})
}
