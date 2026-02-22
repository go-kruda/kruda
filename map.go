package kruda

// Map is a convenience type alias for map[string]any.
// Use it for quick JSON responses without defining a struct.
//
//	c.JSON(kruda.Map{"message": "hello", "ok": true})
type Map = map[string]any
