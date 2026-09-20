package externalbind

//go:generate go run ../internal/bindgen -input input.go -type searchInput -output external_generated.go -func bindSearchInput

// searchInput intentionally carries no validate tags: external generation
// currently supports binder-only inputs (see package doc).
type searchInput struct {
	Q    string `query:"q" default:"all"`
	Page int    `query:"page" default:"1"`
}
