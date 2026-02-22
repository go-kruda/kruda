package kruda

// bindInput parses the request body as JSON into type T.
// Phase 1: JSON only. Phase 2 will add struct tag-based multi-source binding.
// H6 fix: error messages no longer expose internal struct field names.
func bindInput[T any](c *Ctx) (T, error) {
	var v T
	body, err := c.BodyBytes()
	if err != nil {
		return v, BadRequest("failed to read request body")
	}
	if len(body) == 0 {
		return v, BadRequest("empty request body")
	}
	if err := c.app.config.JSONDecoder(body, &v); err != nil {
		return v, BadRequest("invalid request body")
	}
	return v, nil
}
