package kruda

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/go-kruda/kruda/transport"
)

// inputParser is pre-compiled at route registration time.
// It holds all metadata needed to parse a request into type T
// without runtime reflection.
type inputParser struct {
	newFunc     func() reflect.Value // creates new *T
	paramFields []fieldParser        // fields with `param:"name"` tag
	queryFields []fieldParser        // fields with `query:"name"` tag
	hasBody     bool                 // true if any field has `json:"name"` tag
	defaults    []fieldDefault       // fields with `default:"value"` tag
	hasForm     bool                 // true if any field has `form:"name"` tag
	formFields  []formField          // fields with `form:"name"` tag
}

// formField holds pre-compiled metadata for a multipart form field.
type formField struct {
	index    int    // struct field index
	tag      string // form field name
	isFile   bool   // true if field type is *FileUpload
	isMulti  bool   // true if field type is []*FileUpload
	isString bool   // true if field type is string (text form field)
}

// fieldParser holds pre-compiled metadata for a single struct field.
type fieldParser struct {
	index     int                                 // struct field index
	tag       string                              // tag value (param name, query name)
	converter func(string) (reflect.Value, error) // pre-selected string→type converter
}

// fieldDefault holds a pre-parsed default value for a field.
type fieldDefault struct {
	index int           // struct field index
	value reflect.Value // pre-converted default value
}

// buildInputParser reflects on type T once and builds the parser.
// Called at route registration time. All reflection happens here.
func buildInputParser[T any]() *inputParser {
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.Kind() != reflect.Struct {
		panic("kruda: input type must be a struct")
	}

	p := &inputParser{
		newFunc: func() reflect.Value { return reflect.New(t) },
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// Check param tag
		if tag := field.Tag.Get("param"); tag != "" {
			conv := selectConverter(field.Type)
			p.paramFields = append(p.paramFields, fieldParser{
				index: i, tag: tag, converter: conv,
			})
		}

		// Check query tag
		if tag := field.Tag.Get("query"); tag != "" {
			conv := selectConverter(field.Type)
			p.queryFields = append(p.queryFields, fieldParser{
				index: i, tag: tag, converter: conv,
			})
		}

		// Check json tag (body)
		if tag := field.Tag.Get("json"); tag != "" && tag != "-" {
			p.hasBody = true
		}

		// Check default tag
		if def := field.Tag.Get("default"); def != "" {
			conv := selectConverter(field.Type)
			val, err := conv(def)
			if err != nil {
				panic(fmt.Sprintf("kruda: invalid default %q for field %s: %v", def, field.Name, err))
			}
			p.defaults = append(p.defaults, fieldDefault{index: i, value: val})
		}

		// Check form tag
		if tag := field.Tag.Get("form"); tag != "" {
			ff := formField{index: i, tag: tag}
			switch {
			case field.Type == reflect.TypeOf((*FileUpload)(nil)):
				ff.isFile = true
			case field.Type == reflect.TypeOf(([]*FileUpload)(nil)):
				ff.isMulti = true
			case field.Type.Kind() == reflect.String:
				ff.isString = true
			default:
				panic(fmt.Sprintf("kruda: unsupported form field type %s for field %s",
					field.Type, field.Name))
			}
			p.formFields = append(p.formFields, ff)
			p.hasForm = true
		}
	}

	return p
}

// selectConverter returns a string→reflect.Value converter for the given type.
// Selected once at build time, called many times at request time.
func selectConverter(t reflect.Type) func(string) (reflect.Value, error) {
	switch t.Kind() {
	case reflect.String:
		return func(s string) (reflect.Value, error) {
			return reflect.ValueOf(s), nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		bits := t.Bits()
		return func(s string) (reflect.Value, error) {
			n, err := strconv.ParseInt(s, 10, bits)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(n).Convert(t), nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		bits := t.Bits()
		return func(s string) (reflect.Value, error) {
			n, err := strconv.ParseUint(s, 10, bits)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(n).Convert(t), nil
		}
	case reflect.Float32, reflect.Float64:
		bits := t.Bits()
		return func(s string) (reflect.Value, error) {
			f, err := strconv.ParseFloat(s, bits)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(f).Convert(t), nil
		}
	case reflect.Bool:
		return func(s string) (reflect.Value, error) {
			b, err := strconv.ParseBool(s)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(b), nil
		}
	default:
		panic(fmt.Sprintf("kruda: unsupported param/query type: %s", t.Kind()))
	}
}

// parse executes the binding pipeline at request time.
// Uses only pre-compiled data — no reflection on types.
func (p *inputParser) parse(c *Ctx) (reflect.Value, error) {
	// 1. Create new T
	ptr := p.newFunc() // reflect.New(T) → *T
	v := ptr.Elem()    // T (addressable)

	// 2. Set defaults
	for _, d := range p.defaults {
		v.Field(d.index).Set(d.value)
	}

	// 3. Parse form OR JSON body (mutually exclusive)
	if p.hasForm {
		if err := p.parseMultipart(c, v); err != nil {
			return reflect.Value{}, err
		}
	} else if p.hasBody && hasBody(c.method) {
		body, err := c.BodyBytes()
		if err != nil {
			if isBodyTooLarge(err) {
				return reflect.Value{}, NewError(413, "request entity too large", err)
			}
			return reflect.Value{}, BadRequest("failed to read request body")
		}
		if len(body) == 0 {
			return reflect.Value{}, BadRequest("empty request body")
		}
		if err := c.app.config.JSONDecoder(body, ptr.Interface()); err != nil {
			return reflect.Value{}, BadRequest("invalid request body")
		}
	}

	// 4. Parse query params (overwrites body values)
	for _, qf := range p.queryFields {
		raw := c.Query(qf.tag)
		if raw == "" {
			continue // keep default or body value
		}
		val, err := qf.converter(raw)
		if err != nil {
			return reflect.Value{}, BadRequest(
				fmt.Sprintf("invalid query parameter %q: expected %s",
					qf.tag, v.Field(qf.index).Type().Kind()))
		}
		v.Field(qf.index).Set(val)
	}

	// 5. Parse path params (highest priority, overwrites everything)
	for _, pf := range p.paramFields {
		raw := c.Param(pf.tag)
		if raw == "" {
			continue
		}
		val, err := pf.converter(raw)
		if err != nil {
			return reflect.Value{}, BadRequest(
				fmt.Sprintf("invalid path parameter %q: expected %s",
					pf.tag, v.Field(pf.index).Type().Kind()))
		}
		v.Field(pf.index).Set(val)
	}

	return v, nil
}

// hasBody returns true for methods that typically have a request body.
func hasBody(method string) bool {
	return method == "POST" || method == "PUT" || method == "PATCH"
}

// parseMultipart parses multipart/form-data and populates form fields.
func (p *inputParser) parseMultipart(c *Ctx, v reflect.Value) error {
	mp, ok := c.request.(transport.MultipartProvider)
	if !ok {
		return BadRequest("multipart upload not supported by current transport")
	}
	maxBytes := int64(c.app.config.BodyLimit)
	form, err := mp.MultipartForm(maxBytes)
	if err != nil {
		return BadRequest("failed to parse multipart form")
	}
	c.multipartForm = form

	for _, ff := range p.formFields {
		switch {
		case ff.isFile:
			// Single file upload
			headers := form.File[ff.tag]
			if len(headers) == 0 {
				continue // file not provided — let validation catch required
			}
			header := headers[0]
			fu := &FileUpload{
				Name:        header.Filename,
				Size:        header.Size,
				ContentType: header.Header.Get("Content-Type"),
				Header:      header,
			}
			v.Field(ff.index).Set(reflect.ValueOf(fu))

		case ff.isMulti:
			// Multiple file upload
			headers := form.File[ff.tag]
			if len(headers) == 0 {
				continue
			}
			files := make([]*FileUpload, len(headers))
			for i, header := range headers {
				files[i] = &FileUpload{
					Name:        header.Filename,
					Size:        header.Size,
					ContentType: header.Header.Get("Content-Type"),
					Header:      header,
				}
			}
			v.Field(ff.index).Set(reflect.ValueOf(files))

		case ff.isString:
			// Text form field
			vals := form.Value[ff.tag]
			if len(vals) > 0 && vals[0] != "" {
				v.Field(ff.index).Set(reflect.ValueOf(vals[0]))
			}
		}
	}

	return nil
}

// bindInput parses the request body as JSON into type T.
// Phase 1 compatibility: used by untyped handlers via c.Bind().
func bindInput[T any](c *Ctx) (T, error) {
	var v T
	body, err := c.BodyBytes()
	if err != nil {
		if isBodyTooLarge(err) {
			return v, NewError(413, "request entity too large", err)
		}
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
