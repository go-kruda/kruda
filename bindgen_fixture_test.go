package kruda

//go:generate go run ./cmd/bindgen -input bindgen_fixture_test.go -type generatedBinderInput -output bindgen_generated_test.go -func bindGeneratedBinderInput

type generatedBinderInput struct {
	ID     int64   `param:"id" query:"id" default:"7" validate:"min=1"`
	Page   int     `query:"page" default:"1" validate:"min=1,max=1000"`
	Active bool    `query:"active" default:"true"`
	Score  float64 `query:"score" default:"1.5" validate:"min=0,max=100"`
	Name   string  `query:"name" default:"guest" validate:"min=1,max=64"`
}

type generatedBinderOutput struct {
	ID     int64   `json:"id"`
	Page   int     `json:"page"`
	Active bool    `json:"active"`
	Score  float64 `json:"score"`
	Name   string  `json:"name"`
}
