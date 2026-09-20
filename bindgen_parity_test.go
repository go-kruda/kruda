package kruda

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func assertGeneratedBinderParity(t *testing.T, parser *inputParser, params, query map[string]string, wantError string) generatedBinderInput {
	t.Helper()
	want, wantErr := parser.parse(bindCtx("GET", "/users/42", params, query, nil))
	got, gotErr := mustBindGeneratedBinder(t)(bindCtx("GET", "/users/42", params, query, nil))
	if wantError != "" {
		for name, err := range map[string]error{"normal": wantErr, "generated": gotErr} {
			var ke *KrudaError
			if !errors.As(err, &ke) || ke.Code != 400 || ke.Message != wantError || ke.Err != nil {
				t.Fatalf("%s error = %#v, want 400 %q without a wrapped error", name, err, wantError)
			}
		}
		if got.IsValid() || want.IsValid() {
			t.Fatal("failed binding returned a valid input")
		}
		return generatedBinderInput{}
	}
	if wantErr != nil || gotErr != nil {
		t.Fatalf("binding errors: normal=%v generated=%v", wantErr, gotErr)
	}
	if !got.IsValid() || got.Type() != want.Type() || !got.CanAddr() {
		t.Fatalf("generated input must be addressable %v, got %v", want.Type(), got)
	}
	gotInput := got.Interface().(generatedBinderInput)
	wantInput := want.Interface().(generatedBinderInput)
	if gotInput.ID != wantInput.ID || gotInput.Page != wantInput.Page || gotInput.Active != wantInput.Active || gotInput.Name != wantInput.Name {
		t.Fatalf("generated=%+v normal=%+v", gotInput, wantInput)
	}
	if !(math.IsNaN(gotInput.Score) && math.IsNaN(wantInput.Score)) && math.Float64bits(gotInput.Score) != math.Float64bits(wantInput.Score) {
		t.Fatalf("generated score=%v (%x), normal=%v (%x)", gotInput.Score, math.Float64bits(gotInput.Score), wantInput.Score, math.Float64bits(wantInput.Score))
	}
	return gotInput
}

func TestGeneratedBinderDefaultsAndPrecedence(t *testing.T) {
	parser := buildInputParser[generatedBinderInput]()
	for _, tt := range []struct {
		name      string
		params    map[string]string
		query     map[string]string
		wantID    int64
		wantError string
	}{
		{name: "missing", wantID: 7},
		{name: "empty", params: map[string]string{"id": ""}, query: map[string]string{"id": "", "page": "", "active": "", "score": "", "name": ""}, wantID: 7},
		{name: "query", query: map[string]string{"id": "8"}, wantID: 8},
		{name: "path", params: map[string]string{"id": "9"}, wantID: 9},
		{name: "path_over_query", params: map[string]string{"id": "9"}, query: map[string]string{"id": "8"}, wantID: 9},
		{name: "empty_path_preserves_query", params: map[string]string{"id": ""}, query: map[string]string{"id": "8"}, wantID: 8},
		{name: "invalid_query_before_valid_path", params: map[string]string{"id": "9"}, query: map[string]string{"id": "bad"}, wantError: `invalid query parameter "id": expected int64`},
		{name: "invalid_path_after_query", params: map[string]string{"id": "bad"}, query: map[string]string{"id": "8"}, wantError: `invalid path parameter "id": expected int64`},
		{name: "query_fields_before_path_fields", params: map[string]string{"id": "bad"}, query: map[string]string{"page": "bad"}, wantError: `invalid query parameter "page": expected int`},
		{name: "query_declaration_order", query: map[string]string{"id": "bad", "page": "bad", "active": "bad", "score": "bad"}, wantError: `invalid query parameter "id": expected int64`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := assertGeneratedBinderParity(t, parser, tt.params, tt.query, tt.wantError)
			if tt.wantError == "" {
				want := generatedBinderInput{ID: tt.wantID, Page: 1, Active: true, Score: 1.5, Name: "guest"}
				if got != want {
					t.Fatalf("input=%+v, want %+v", got, want)
				}
			}
		})
	}
}

func TestGeneratedValidatorParity(t *testing.T) {
	v := NewValidator()
	validators := buildValidators[generatedBinderInput](v)
	generated := makeBindGeneratedBinderInputValidator(validators)
	if generated == nil {
		t.Fatal("generated validator was not selected")
	}
	boxed := append([]fieldValidator(nil), validators...)
	for i := range boxed {
		boxed[i].checks = nil
	}

	valid := generatedBinderInput{ID: 1, Page: 1, Score: 0, Name: "a"}
	cases := []struct {
		name  string
		input generatedBinderInput
	}{
		{"minimums", valid},
		{"maximums", generatedBinderInput{ID: math.MaxInt64, Page: 1000, Score: 100, Name: strings.Repeat("x", 64)}},
		{"id_below_min", generatedBinderInput{ID: 0, Page: 1, Score: 0, Name: "a"}},
		{"page_below_min", generatedBinderInput{ID: 1, Page: 0, Score: 0, Name: "a"}},
		{"page_above_max", generatedBinderInput{ID: 1, Page: 1001, Score: 0, Name: "a"}},
		{"score_below_min", generatedBinderInput{ID: 1, Page: 1, Score: -math.SmallestNonzeroFloat64, Name: "a"}},
		{"score_above_max", generatedBinderInput{ID: 1, Page: 1, Score: 101, Name: "a"}},
		{"score_nan", generatedBinderInput{ID: 1, Page: 1, Score: math.NaN(), Name: "a"}},
		{"score_positive_inf", generatedBinderInput{ID: 1, Page: 1, Score: math.Inf(1), Name: "a"}},
		{"score_negative_inf", generatedBinderInput{ID: 1, Page: 1, Score: math.Inf(-1), Name: "a"}},
		{"name_empty", generatedBinderInput{ID: 1, Page: 1, Score: 0}},
		{"name_multibyte_within_max", generatedBinderInput{ID: 1, Page: 1, Score: 0, Name: strings.Repeat("ก", 21)}},
		{"name_multibyte_above_max", generatedBinderInput{ID: 1, Page: 1, Score: 0, Name: strings.Repeat("ก", 22)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generated(&tc.input)
			want := validate(boxed, reflect.ValueOf(&tc.input).Elem(), v.messages) == nil
			if got != want {
				t.Fatalf("generated validation = %t, boxed validation = %t for %+v", got, want, tc.input)
			}
		})
	}
}

func TestGeneratedBinderScalarParity(t *testing.T) {
	parser := buildInputParser[generatedBinderInput]()
	maxInt := int(^uint(0) >> 1)
	for _, field := range []struct {
		key     string
		kind    string
		valid   []string
		invalid []string
	}{
		{"id", "int64", []string{"0", "-0", "+42", "00042", "-9223372036854775808", "9223372036854775807"}, []string{"-9223372036854775809", "9223372036854775808", "0x10", "1_000", "1.0", "1e2", " 1", "1 ", "１"}},
		{"page", "int", []string{"0", "-0", "+42", "00042", strconv.Itoa(maxInt), strconv.Itoa(-maxInt - 1)}, []string{"-9223372036854775809", "9223372036854775808", "0x10", "1_000", "1.0", " 1", "1 ", "１"}},
		{"active", "bool", []string{"1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False"}, []string{"yes", "no", "2", " true", "true ", "TrUe"}},
		{"score", "float64", []string{"0", "-0", "+1.5", "1e-3", "0x1.8p+1", "1.7976931348623157e308", "5e-324", "NaN", "+Inf", "-Inf"}, []string{"1e9999", "-1e9999", "1x", " 1", "1 "}},
		{"name", "string", []string{"guest", " ", "ไทย", "a+b%20c", "x\x00y", "\xff"}, nil},
	} {
		t.Run(field.key, func(t *testing.T) {
			for _, raw := range field.valid {
				t.Run("valid_"+strconv.Quote(raw), func(t *testing.T) {
					assertGeneratedBinderParity(t, parser, nil, map[string]string{field.key: raw}, "")
					if field.key == "id" {
						assertGeneratedBinderParity(t, parser, map[string]string{"id": raw}, nil, "")
					}
				})
			}
			for _, raw := range field.invalid {
				t.Run("invalid_"+strconv.Quote(raw), func(t *testing.T) {
					assertGeneratedBinderParity(t, parser, nil, map[string]string{field.key: raw}, fmt.Sprintf("invalid query parameter %q: expected %s", field.key, field.kind))
					if field.key == "id" {
						assertGeneratedBinderParity(t, parser, map[string]string{"id": raw}, nil, `invalid path parameter "id": expected int64`)
					}
				})
			}
		})
	}
}

func generatedBinderParityHandler(app *App, generated bool, handler func(*C[generatedBinderInput]) (*generatedBinderInput, error)) HandlerFunc {
	if generated {
		return buildTypedHandlerWithBinder[generatedBinderInput, generatedBinderInput](app, "GET", "/users/:id", handler, nil, bindGeneratedBinderInputAttested())
	}
	return buildTypedHandler[generatedBinderInput, generatedBinderInput](app, "GET", "/users/:id", handler, nil)
}

func TestGeneratedBinderRetainedOnParseInput(t *testing.T) {
	for _, generated := range []bool{false, true} {
		t.Run(fmt.Sprintf("generated=%t", generated), func(t *testing.T) {
			app := New()
			var retained []*generatedBinderInput
			app.OnParse(func(_ *Ctx, value any) error {
				in := value.(*generatedBinderInput)
				in.Name = "parsed-" + in.Name
				retained = append(retained, in)
				return nil
			})
			app.OnParse(func(_ *Ctx, value any) error {
				if value.(*generatedBinderInput) != retained[len(retained)-1] {
					t.Fatal("OnParse hooks received different input pointers")
				}
				return nil
			})
			app.Get("/users/:id", generatedBinderParityHandler(app, generated, func(c *C[generatedBinderInput]) (*generatedBinderInput, error) {
				in := retained[len(retained)-1]
				if c.In.Name != in.Name {
					t.Fatalf("handler missed OnParse mutation: %+v, retained %+v", c.In, in)
				}
				c.In.Name = "handled"
				if in.Name == "handled" {
					t.Fatal("handler mutation changed retained OnParse input")
				}
				return nil, nil
			}))
			app.Compile()
			for _, name := range []string{"first", "second"} {
				resp := newMockResponse()
				app.ServeKruda(resp, &mockRequest{method: "GET", path: "/users/42", query: map[string]string{"name": name}})
				if resp.statusCode != 204 || len(resp.body) != 0 {
					t.Fatalf("response=%d %s, want empty 204", resp.statusCode, resp.body)
				}
			}
			if len(retained) != 2 || retained[0] == retained[1] || retained[0].Name != "parsed-first" || retained[1].Name != "parsed-second" {
				t.Fatalf("retained inputs changed across requests: %+v", retained)
			}
		})
	}
}

func TestGeneratedBinderHandlerPipelineParity(t *testing.T) {
	for _, tt := range []struct {
		name          string
		id            string
		query         map[string]string
		action        string
		wantStatus    int
		wantParse     bool
		wantValidate  bool
		wantHandler   bool
		wantFieldErrs int
	}{
		{name: "defaults", id: "42", wantStatus: 200, wantParse: true, wantValidate: true, wantHandler: true},
		{name: "all_fields", id: "43", query: map[string]string{"id": "9", "page": "12", "active": "false", "score": "2.5", "name": "Ada"}, wantStatus: 200, wantParse: true, wantValidate: true, wantHandler: true},
		{name: "conversion_error", id: "42", query: map[string]string{"page": "bad"}, wantStatus: 400},
		{name: "validation_errors", id: "0", query: map[string]string{"page": "0", "score": "101", "name": strings.Repeat("x", 65)}, wantStatus: 422, wantParse: true, wantValidate: true, wantFieldErrs: 4},
		{name: "hook_repairs_before_validation", id: "42", query: map[string]string{"page": "0"}, action: "repair", wantStatus: 200, wantParse: true, wantValidate: true, wantHandler: true},
		{name: "hook_invalidates_before_validation", id: "42", action: "invalidate", wantStatus: 422, wantParse: true, wantValidate: true, wantFieldErrs: 1},
		{name: "parse_hook_error", id: "42", action: "reject", wantStatus: 400, wantParse: true},
		{name: "middleware_short_circuit", id: "42", action: "short", wantStatus: 202},
		{name: "before_handle_error", id: "42", action: "before-error", wantStatus: 403},
		{name: "handler_error", id: "42", action: "handler-error", wantStatus: 409, wantParse: true, wantValidate: true, wantHandler: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			type result struct {
				response        *mockResponseWriter
				trace           []string
				validationCalls int
				fieldErrors     []FieldError
				input           *generatedBinderInput
			}
			run := func(generated bool) result {
				got := result{response: newMockResponse()}
				app := New()
				minRule := app.config.Validator.rules["min"]
				app.config.Validator.Register("min", func(value any, param string) bool {
					got.validationCalls++
					return minRule(value, param)
				})
				app.OnRequest(func(c *Ctx) error {
					got.trace = append(got.trace, "request")
					c.SetHeader("X-Request", "seen")
					return nil
				})
				app.BeforeHandle(func(_ *Ctx) error {
					got.trace = append(got.trace, "before")
					if tt.action == "before-error" {
						return Forbidden("blocked before handler")
					}
					return nil
				})
				app.Use(func(c *Ctx) error {
					got.trace = append(got.trace, "middleware-in")
					c.SetHeader("X-Middleware", "seen")
					if tt.action == "short" {
						return c.Status(202).Text("short circuit")
					}
					err := c.Next()
					got.trace = append(got.trace, "middleware-out")
					return err
				})
				parseError := BadRequest("parse hook rejected input")
				app.OnParse(func(_ *Ctx, value any) error {
					got.trace = append(got.trace, "parse-1")
					in := value.(*generatedBinderInput)
					switch tt.action {
					case "repair":
						in.Page = 2
					case "invalidate":
						in.Page = 0
					case "reject":
						return parseError
					}
					return nil
				})
				app.OnParse(func(_ *Ctx, value any) error {
					got.trace = append(got.trace, "parse-2")
					if tt.action == "repair" && value.(*generatedBinderInput).Page != 2 {
						t.Fatal("second OnParse hook missed first hook mutation")
					}
					return nil
				})
				app.AfterHandle(func(_ *Ctx) error {
					got.trace = append(got.trace, "after")
					return nil
				})
				app.OnError(func(_ *Ctx, err error) {
					got.trace = append(got.trace, "error")
					var ve *ValidationError
					if errors.As(err, &ve) {
						got.fieldErrors = append([]FieldError(nil), ve.Errors...)
					}
					if tt.action == "reject" && !errors.Is(err, parseError) {
						t.Fatalf("OnError lost OnParse error identity: %v", err)
					}
				})
				app.OnResponse(func(_ *Ctx) error {
					got.trace = append(got.trace, "response")
					return nil
				})
				app.Get("/users/:id", generatedBinderParityHandler(app, generated, func(c *C[generatedBinderInput]) (*generatedBinderInput, error) {
					got.trace = append(got.trace, "handler")
					in := c.In
					got.input = &in
					if tt.action == "handler-error" {
						return nil, Conflict("handler rejected input")
					}
					c.SetHeader("X-Handler", "seen")
					return &in, nil
				}))
				app.Compile()
				app.ServeKruda(got.response, &mockRequest{method: "GET", path: "/users/" + tt.id, query: tt.query})
				return got
			}
			want := run(false)
			got := run(true)
			if got.response.statusCode != tt.wantStatus || want.response.statusCode != tt.wantStatus {
				t.Fatalf("status: generated=%d normal=%d, want %d; bodies: %s / %s", got.response.statusCode, want.response.statusCode, tt.wantStatus, got.response.body, want.response.body)
			}
			if !bytes.Equal(got.response.body, want.response.body) || !reflect.DeepEqual(got.response.headers, want.response.headers) {
				t.Fatalf("response mismatch: generated=%+v normal=%+v", got.response, want.response)
			}
			if !reflect.DeepEqual(got.trace, want.trace) || !reflect.DeepEqual(got.fieldErrors, want.fieldErrors) || !reflect.DeepEqual(got.input, want.input) || got.validationCalls != want.validationCalls {
				t.Fatalf("pipeline mismatch: generated=%+v normal=%+v", got, want)
			}
			parsed := false
			secondParse := false
			for _, event := range got.trace {
				parsed = parsed || event == "parse-1"
				secondParse = secondParse || event == "parse-2"
			}
			if parsed != tt.wantParse || (got.validationCalls > 0) != tt.wantValidate || (got.input != nil) != tt.wantHandler || len(got.fieldErrors) != tt.wantFieldErrs {
				t.Fatalf("unexpected pipeline stages: %+v", got)
			}
			if tt.action == "reject" && secondParse {
				t.Fatal("OnParse error did not stop subsequent hooks")
			}
			if len(got.trace) == 0 || got.trace[len(got.trace)-1] != "response" || got.response.headers.Get("X-Request") != "seen" {
				t.Fatalf("lifecycle hooks did not complete: %+v", got)
			}
		})
	}
}
