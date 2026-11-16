package kruda

import (
	"reflect"
	"testing"
)

type setterNamedInt int16
type setterNamedUint uint32
type setterNamedFloat float32
type setterNamedString string
type setterNamedBool bool

func TestSelectFieldSetterConverterParity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		value  any
		inputs []string
	}{
		{"string", "", []string{"hello", ""}},
		{"int", int(0), []string{"-42", "invalid", "18446744073709551616"}},
		{"int8", int8(0), []string{"-128", "127", "128", "invalid"}},
		{"int16", int16(0), []string{"-32768", "32767", "32768", "invalid"}},
		{"int32", int32(0), []string{"-2147483648", "2147483647", "2147483648", "invalid"}},
		{"int64", int64(0), []string{"-9223372036854775808", "9223372036854775807", "9223372036854775808", "invalid"}},
		{"uint", uint(0), []string{"42", "-1", "18446744073709551616"}},
		{"uint8", uint8(0), []string{"255", "256", "-1", "invalid"}},
		{"uint16", uint16(0), []string{"65535", "65536", "invalid"}},
		{"uint32", uint32(0), []string{"4294967295", "4294967296", "invalid"}},
		{"uint64", uint64(0), []string{"18446744073709551615", "18446744073709551616", "invalid"}},
		{"float32", float32(0), []string{"1.00000006", "3.4028235e38", "3.5e38", "invalid"}},
		{"float64", float64(0), []string{"1.0000000000000002", "1.7976931348623157e308", "1.8e308", "invalid"}},
		{"bool", false, []string{"1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False", "yes", ""}},
		{"named int", setterNamedInt(0), []string{"-32768", "32767", "32768", "invalid"}},
		{"named uint", setterNamedUint(0), []string{"4294967295", "4294967296", "invalid"}},
		{"named float", setterNamedFloat(0), []string{"1.00000006", "3.5e38", "invalid"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typ := reflect.TypeOf(tc.value)
			set := selectFieldSetter(typ)
			convert := selectConverter(typ)
			for _, raw := range tc.inputs {
				t.Run(raw, func(t *testing.T) {
					got := reflect.New(typ).Elem()
					initial := reflect.New(typ).Elem()
					if typ.Kind() == reflect.String {
						initial.SetString("unchanged")
						got.Set(initial)
					} else if typ.Kind() == reflect.Bool {
						initial.SetBool(true)
						got.Set(initial)
					} else if typ.Kind() == reflect.Float32 || typ.Kind() == reflect.Float64 {
						initial.SetFloat(7)
						got.Set(initial)
					} else if typ.Kind() >= reflect.Uint && typ.Kind() <= reflect.Uint64 {
						initial.SetUint(7)
						got.Set(initial)
					} else {
						initial.SetInt(7)
						got.Set(initial)
					}
					want, wantErr := convert(raw)
					gotErr := set(got, raw)
					if (gotErr == nil) != (wantErr == nil) || gotErr != nil && gotErr.Error() != wantErr.Error() {
						t.Fatalf("input %q: setter error %v, converter error %v", raw, gotErr, wantErr)
					}
					if gotErr != nil {
						if !reflect.DeepEqual(got.Interface(), initial.Interface()) {
							t.Fatalf("input %q mutated field to %v on error", raw, got.Interface())
						}
						return
					}
					if !reflect.DeepEqual(got.Interface(), want.Interface()) {
						t.Fatalf("input %q: got %v (%T), want %v (%T)", raw, got.Interface(), got.Interface(), want.Interface(), want.Interface())
					}
				})
			}
		})
	}
}

func TestSelectFieldSetterNamedStringBoolLegacyPanic(t *testing.T) {
	for _, tc := range []struct {
		value any
		raw   string
	}{
		{setterNamedString(""), "name"},
		{setterNamedBool(false), "true"},
	} {
		t.Run(reflect.TypeOf(tc.value).String(), func(t *testing.T) {
			typ := reflect.TypeOf(tc.value)
			old := func() (panicked bool) {
				defer func() { panicked = recover() != nil }()
				value, err := selectConverter(typ)(tc.raw)
				if err != nil {
					t.Fatal(err)
				}
				reflect.New(typ).Elem().Set(value)
				return false
			}()
			new := func() (panicked bool) {
				defer func() { panicked = recover() != nil }()
				if err := selectFieldSetter(typ)(reflect.New(typ).Elem(), tc.raw); err != nil {
					t.Fatal(err)
				}
				return false
			}()
			if !old || !new {
				t.Fatalf("legacy panic = %t, setter panic = %t; want both", old, new)
			}
		})
	}
}

func TestFieldSetterParsePrecedence(t *testing.T) {
	type input struct {
		Value int `default:"1" json:"value" query:"value" param:"value"`
	}
	p := buildInputParser[input]()
	for _, tc := range []struct {
		name  string
		query string
		param string
		body  string
		want  int
	}{
		{"default", "", "", `{}`, 1},
		{"body", "", "", `{"value":2}`, 2},
		{"query", "3", "", `{"value":2}`, 3},
		{"path", "3", "4", `{"value":2}`, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := bindCtx("POST", "/items", map[string]string{"value": tc.param}, map[string]string{"value": tc.query}, []byte(tc.body))
			v, err := p.parse(c)
			if err != nil {
				t.Fatal(err)
			}
			if got := v.Interface().(input).Value; got != tc.want {
				t.Fatalf("Value = %d, want %d", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		query string
		param string
		want  string
	}{
		{"bad", "bad", `invalid query parameter "value": expected int`},
		{"3", "bad", `invalid path parameter "value": expected int`},
	} {
		c := bindCtx("POST", "/items", map[string]string{"value": tc.param}, map[string]string{"value": tc.query}, []byte(`{"value":2}`))
		_, err := p.parse(c)
		if err == nil || err.Error() != tc.want {
			t.Fatalf("query %q, path %q: error = %v, want %q", tc.query, tc.param, err, tc.want)
		}
	}
}

func TestFieldSetterRetainedStringsOnParse(t *testing.T) {
	type input struct {
		ID   string `param:"id"`
		Name string `query:"name"`
	}
	app := New()
	var retained []*input
	app.OnParse(func(_ *Ctx, value any) error {
		retained = append(retained, value.(*input))
		return nil
	})
	Get[input, input](app, "/users/:id", func(c *C[input]) (*input, error) {
		return &c.In, nil
	})
	app.Compile()
	for _, tc := range []struct{ id, name string }{{"first", "alice"}, {"second", "bob"}} {
		resp := newMockResponse()
		app.ServeKruda(resp, &mockRequest{method: "GET", path: "/users/" + tc.id, query: map[string]string{"name": tc.name}})
		if resp.statusCode != 200 {
			t.Fatalf("response status = %d, want 200", resp.statusCode)
		}
	}
	if len(retained) != 2 || retained[0].ID != "first" || retained[0].Name != "alice" || retained[1].ID != "second" || retained[1].Name != "bob" {
		t.Fatalf("retained OnParse inputs = %+v", retained)
	}
}
