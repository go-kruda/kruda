package externalbind

//go:generate go run ../cmd/bindgen -input input.go -type searchInput -output external_generated.go -func bindSearchInput

// searchInput exercises the full external path: generated single-pass
// binding plus compiled validation, selected through the public route
// option. Both rules use built-in min/max so the factory engages; see
// the package doc for the unsupported-rule fallback.
type searchInput struct {
	Q    string `query:"q" default:"all" validate:"min=1,max=64"`
	Page int    `query:"page" default:"1" validate:"min=1,max=1000"`
}

// searchOutput is the typed response for the external route test.
type searchOutput struct {
	Q    string `json:"q"`
	Page int    `json:"page"`
}
