package kruda

import (
	"reflect"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// OpenAPI 3.1.0 types
// ---------------------------------------------------------------------------

// openAPISpec represents the complete OpenAPI 3.1.0 document.
type openAPISpec struct {
	OpenAPI    string                  `json:"openapi"`
	Info       openAPIInfo             `json:"info"`
	Paths      map[string]*openAPIPath `json:"paths"`
	Components *openAPIComponents      `json:"components,omitempty"`
	Tags       []openAPITagDef         `json:"tags,omitempty"`
}

type openAPIInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

type openAPIPath struct {
	Get    *openAPIOperation `json:"get,omitempty"`
	Post   *openAPIOperation `json:"post,omitempty"`
	Put    *openAPIOperation `json:"put,omitempty"`
	Delete *openAPIOperation `json:"delete,omitempty"`
	Patch  *openAPIOperation `json:"patch,omitempty"`
}

type openAPIOperation struct {
	Summary     string                      `json:"summary,omitempty"`
	Description string                      `json:"description,omitempty"`
	Tags        []string                    `json:"tags,omitempty"`
	Parameters  []openAPIParameter          `json:"parameters,omitempty"`
	RequestBody *openAPIRequestBody         `json:"requestBody,omitempty"`
	Responses   map[string]*openAPIResponse `json:"responses"`
}

type openAPIParameter struct {
	Name     string     `json:"name"`
	In       string     `json:"in"`
	Required bool       `json:"required,omitempty"`
	Schema   *schemaRef `json:"schema"`
}

type openAPIRequestBody struct {
	Required bool                         `json:"required,omitempty"`
	Content  map[string]*openAPIMediaType `json:"content"`
}

type openAPIMediaType struct {
	Schema *schemaRef `json:"schema"`
}

type openAPIResponse struct {
	Description string                       `json:"description"`
	Content     map[string]*openAPIMediaType `json:"content,omitempty"`
}

type openAPIComponents struct {
	Schemas map[string]*schemaRef `json:"schemas,omitempty"`
}

type openAPITagDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// schemaRef is a JSON Schema reference or inline schema.
type schemaRef struct {
	Ref        string                `json:"$ref,omitempty"`
	Type       string                `json:"type,omitempty"`
	Format     string                `json:"format,omitempty"`
	Properties map[string]*schemaRef `json:"properties,omitempty"`
	Required   []string              `json:"required,omitempty"`
	Items      *schemaRef            `json:"items,omitempty"`
	Enum       []string              `json:"enum,omitempty"`
	Minimum    *float64              `json:"minimum,omitempty"`
	Maximum    *float64              `json:"maximum,omitempty"`
	MinLength  *int                  `json:"minLength,omitempty"`
	MaxLength  *int                  `json:"maxLength,omitempty"`
	Nullable   bool                  `json:"nullable,omitempty"`
}

// routeInfo holds metadata for a single registered typed route.
type routeInfo struct {
	method      string
	path        string
	config      routeConfig
	hasBody     bool
	hasForm     bool
	hasValidate bool
}

// ---------------------------------------------------------------------------
// Schema generation
// ---------------------------------------------------------------------------

// generateSchema converts a Go reflect.Type to a JSON Schema.
func generateSchema(t reflect.Type, components map[string]*schemaRef) *schemaRef {
	if t.Kind() == reflect.Ptr {
		inner := generateSchema(t.Elem(), components)
		inner.Nullable = true
		return inner
	}

	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		return &schemaRef{
			Type:  "array",
			Items: generateSchema(t.Elem(), components),
		}
	}

	switch t.Kind() {
	case reflect.String:
		return &schemaRef{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &schemaRef{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &schemaRef{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &schemaRef{Type: "number"}
	case reflect.Bool:
		return &schemaRef{Type: "boolean"}
	}

	if t.Kind() != reflect.Struct {
		return &schemaRef{Type: "object"}
	}

	name := t.Name()
	if name == "" {
		name = "Anonymous"
	}

	// NOTE: uses short struct name as component key. Structs with the same name
	// from different packages (e.g. user.CreateReq vs product.CreateReq) will
	// collide. A future improvement could detect this via t.PkgPath() and suffix
	// with the package name when a collision is found.
	refPath := "#/components/schemas/" + name
	if _, exists := components[name]; exists {
		return &schemaRef{Ref: refPath}
	}

	schema := &schemaRef{
		Type:       "object",
		Properties: make(map[string]*schemaRef),
	}
	components[name] = schema

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		propName := field.Tag.Get("json")
		if propName == "" || propName == "-" {
			continue
		}
		if idx := strings.IndexByte(propName, ','); idx != -1 {
			propName = propName[:idx]
		}

		propSchema := generateSchema(field.Type, components)

		if vtag := field.Tag.Get("validate"); vtag != "" {
			applyValidationConstraints(propSchema, vtag, field.Type)
			if containsRule(vtag, "required") {
				schema.Required = append(schema.Required, propName)
			}
		}

		schema.Properties[propName] = propSchema
	}

	return &schemaRef{Ref: refPath}
}

// applyValidationConstraints maps validate tags to JSON Schema constraints.
func applyValidationConstraints(s *schemaRef, vtag string, t reflect.Type) {
	for _, rule := range strings.Split(vtag, ",") {
		name, param, _ := strings.Cut(strings.TrimSpace(rule), "=")
		switch name {
		case "min":
			if n, err := strconv.ParseFloat(param, 64); err == nil {
				if t.Kind() == reflect.String {
					intN := int(n)
					s.MinLength = &intN
				} else {
					s.Minimum = &n
				}
			}
		case "max":
			if n, err := strconv.ParseFloat(param, 64); err == nil {
				if t.Kind() == reflect.String {
					intN := int(n)
					s.MaxLength = &intN
				} else {
					s.Maximum = &n
				}
			}
		case "oneof":
			s.Enum = strings.Fields(param)
		case "email":
			s.Format = "email"
		case "url":
			s.Format = "uri"
		case "uuid":
			s.Format = "uuid"
		}
	}
}

// containsRule checks if a validate tag string contains a specific rule.
func containsRule(vtag, rule string) bool {
	for _, r := range strings.Split(vtag, ",") {
		name, _, _ := strings.Cut(strings.TrimSpace(r), "=")
		if name == rule {
			return true
		}
	}
	return false
}

// convertPath converts Kruda path format to OpenAPI format: ":id" → "{id}".
// Strips regex constraints (":id<[0-9]+>" → "{id}") and optional markers (":id?" → "{id}").
func convertPath(path string) string {
	var b strings.Builder
	i := 0
	for i < len(path) {
		if path[i] == ':' {
			b.WriteByte('{')
			i++
			for i < len(path) && path[i] != '/' && path[i] != '<' && path[i] != '?' {
				b.WriteByte(path[i])
				i++
			}
			// Skip regex constraint <...>
			if i < len(path) && path[i] == '<' {
				depth := 1
				i++ // skip '<'
				for i < len(path) && depth > 0 {
					if path[i] == '<' {
						depth++
					} else if path[i] == '>' {
						depth--
					}
					i++
				}
			}
			// Skip optional marker '?'
			if i < len(path) && path[i] == '?' {
				i++
			}
			b.WriteByte('}')
		} else {
			b.WriteByte(path[i])
			i++
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Spec builder
// ---------------------------------------------------------------------------

// buildOpenAPISpec generates the complete OpenAPI 3.1.0 document from route metadata.
func (app *App) buildOpenAPISpec() ([]byte, error) {
	spec := openAPISpec{
		OpenAPI: "3.1.0",
		Info:    app.config.openAPIInfo,
		Paths:   make(map[string]*openAPIPath),
		Tags:    app.config.openAPITags,
	}

	components := make(map[string]*schemaRef)

	for _, ri := range app.routeInfos {
		oaPath := convertPath(ri.path)

		pathItem, ok := spec.Paths[oaPath]
		if !ok {
			pathItem = &openAPIPath{}
			spec.Paths[oaPath] = pathItem
		}

		op := buildOperation(ri, components)

		switch ri.method {
		case "GET":
			pathItem.Get = op
		case "POST":
			pathItem.Post = op
		case "PUT":
			pathItem.Put = op
		case "DELETE":
			pathItem.Delete = op
		case "PATCH":
			pathItem.Patch = op
		}
	}

	if len(components) > 0 {
		spec.Components = &openAPIComponents{Schemas: components}
	}

	return app.config.JSONEncoder(spec)
}

// buildOperation creates an OpenAPI operation from route info.
func buildOperation(ri routeInfo, components map[string]*schemaRef) *openAPIOperation {
	op := &openAPIOperation{
		Description: ri.config.description,
		Tags:        ri.config.tags,
		Responses:   make(map[string]*openAPIResponse),
	}

	inType := ri.config.inType
	outType := ri.config.outType

	if inType != nil && inType.Kind() == reflect.Struct {
		for i := 0; i < inType.NumField(); i++ {
			field := inType.Field(i)
			if tag := field.Tag.Get("param"); tag != "" {
				op.Parameters = append(op.Parameters, openAPIParameter{
					Name:     tag,
					In:       "path",
					Required: true,
					Schema:   generateSchema(field.Type, components),
				})
			}
			if tag := field.Tag.Get("query"); tag != "" {
				op.Parameters = append(op.Parameters, openAPIParameter{
					Name:   tag,
					In:     "query",
					Schema: generateSchema(field.Type, components),
				})
			}
		}
	}

	if ri.hasBody || ri.hasForm {
		contentType := "application/json"
		if ri.hasForm {
			contentType = "multipart/form-data"
		}
		if inType != nil {
			bodySchema := generateSchema(inType, components)
			op.RequestBody = &openAPIRequestBody{
				Required: true,
				Content: map[string]*openAPIMediaType{
					contentType: {Schema: bodySchema},
				},
			}
		}
	}

	if outType != nil {
		outSchema := generateSchema(outType, components)
		op.Responses["200"] = &openAPIResponse{
			Description: "Successful response",
			Content: map[string]*openAPIMediaType{
				"application/json": {Schema: outSchema},
			},
		}
	}

	if ri.hasValidate {
		op.Responses["422"] = &openAPIResponse{
			Description: "Validation failed",
		}
	}

	op.Responses["default"] = &openAPIResponse{
		Description: "Error response",
	}

	return op
}
