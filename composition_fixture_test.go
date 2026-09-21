package kruda

//go:generate go run ./cmd/bindgen -input composition_fixture_test.go -type composition5Input -output composition_5_generated_test.go -func bindComposition5Input

type composition5Input struct {
	ID     int64   `param:"id" query:"id" default:"7" validate:"min=1"`
	Page   int     `query:"page" default:"1" validate:"min=1,max=1000"`
	Active bool    `query:"active" default:"true"`
	Score  float64 `query:"score" default:"1.5" validate:"min=0,max=100"`
	Name   string  `query:"name" default:"guest" validate:"min=1,max=64"`
}

//go:generate go run ./cmd/bindgen -input composition_fixture_test.go -type composition10Input -output composition_10_generated_test.go -func bindComposition10Input

type composition10Input struct {
	ID      int64   `param:"id" query:"id" default:"7" validate:"min=1"`
	Page    int     `query:"page" default:"1" validate:"min=1,max=1000"`
	Active  bool    `query:"active" default:"true"`
	Score   float64 `query:"score" default:"1.5" validate:"min=0,max=100"`
	Name    string  `query:"name" default:"guest" validate:"min=1,max=64"`
	ID2     int64   `query:"id2" default:"7" validate:"min=1"`
	Page2   int     `query:"page2" default:"1" validate:"min=1,max=1000"`
	Active2 bool    `query:"active2" default:"true"`
	Score2  float64 `query:"score2" default:"1.5" validate:"min=0,max=100"`
	Name2   string  `query:"name2" default:"guest" validate:"min=1,max=64"`
}

//go:generate go run ./cmd/bindgen -input composition_fixture_test.go -type composition30Input -output composition_30_generated_test.go -func bindComposition30Input

type composition30Input struct {
	ID      int64   `param:"id" query:"id" default:"7" validate:"min=1"`
	Page    int     `query:"page" default:"1" validate:"min=1,max=1000"`
	Active  bool    `query:"active" default:"true"`
	Score   float64 `query:"score" default:"1.5" validate:"min=0,max=100"`
	Name    string  `query:"name" default:"guest" validate:"min=1,max=64"`
	ID2     int64   `query:"id2" default:"7" validate:"min=1"`
	Page2   int     `query:"page2" default:"1" validate:"min=1,max=1000"`
	Active2 bool    `query:"active2" default:"true"`
	Score2  float64 `query:"score2" default:"1.5" validate:"min=0,max=100"`
	Name2   string  `query:"name2" default:"guest" validate:"min=1,max=64"`
	ID3     int64   `query:"id3" default:"7" validate:"min=1"`
	Page3   int     `query:"page3" default:"1" validate:"min=1,max=1000"`
	Active3 bool    `query:"active3" default:"true"`
	Score3  float64 `query:"score3" default:"1.5" validate:"min=0,max=100"`
	Name3   string  `query:"name3" default:"guest" validate:"min=1,max=64"`
	ID4     int64   `query:"id4" default:"7" validate:"min=1"`
	Page4   int     `query:"page4" default:"1" validate:"min=1,max=1000"`
	Active4 bool    `query:"active4" default:"true"`
	Score4  float64 `query:"score4" default:"1.5" validate:"min=0,max=100"`
	Name4   string  `query:"name4" default:"guest" validate:"min=1,max=64"`
	ID5     int64   `query:"id5" default:"7" validate:"min=1"`
	Page5   int     `query:"page5" default:"1" validate:"min=1,max=1000"`
	Active5 bool    `query:"active5" default:"true"`
	Score5  float64 `query:"score5" default:"1.5" validate:"min=0,max=100"`
	Name5   string  `query:"name5" default:"guest" validate:"min=1,max=64"`
	ID6     int64   `query:"id6" default:"7" validate:"min=1"`
	Page6   int     `query:"page6" default:"1" validate:"min=1,max=1000"`
	Active6 bool    `query:"active6" default:"true"`
	Score6  float64 `query:"score6" default:"1.5" validate:"min=0,max=100"`
	Name6   string  `query:"name6" default:"guest" validate:"min=1,max=64"`
}

type compositionPostInput struct {
	Name  string `json:"name" validate:"min=1,max=64"`
	Count int    `json:"count" validate:"min=1,max=1000"`
}
