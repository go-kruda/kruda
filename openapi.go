package kruda

import (
	"reflect"
	"strconv"
	"strings"
)

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
	Security    []map[string][]string       `json:"security,omitempty"`
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
	Schema  *schemaRef `json:"schema"`
	Example any        `json:"example,omitempty"`
}

type openAPIResponse struct {
	Description string                       `json:"description"`
	Content     map[string]*openAPIMediaType `json:"content,omitempty"`
}

type openAPIComponents struct {
	Schemas         map[string]*schemaRef            `json:"schemas,omitempty"`
	SecuritySchemes map[string]OpenAPISecurityScheme `json:"securitySchemes,omitempty"`
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

	// pkgPath tracks the originating package for collision detection.
	// Not serialized to JSON — internal bookkeeping only.
	pkgPath string `json:"-"`
}

// routeInfo holds metadata for a single registered typed route.
type routeInfo struct {
	method      string
	path        string
	config      routeConfig
	hasBody     bool
	hasForm     bool
	hasValidate bool
	resourceOp  *resourceOp // non-nil for kruda.Resource routes; nil for typed routes
}

// resourceOp carries the explicit OpenAPI metadata for a single auto-CRUD
// operation registered by kruda.Resource. Rendering is fully determined by
// these fields (no kind discriminator). Consumed by buildOperation.
type resourceOp struct {
	idParam        string       // path-param name (cfg.idParam); "" for list/create
	idType         reflect.Type // ID type for the path-param schema; nil for list/create
	bodyType       reflect.Type // T for create/update; nil otherwise
	respType       reflect.Type // T (get/create/update) | ResourceList[T] (list) | nil (delete → 204)
	successCode    string       // "200" | "201" | "204"
	needsListQuery bool         // emit page/limit integer query params
	hasValidate    bool         // T has validate tags AND a Validator is configured → add 422
}

// generateSchema converts a Go reflect.Type to a JSON Schema.
func generateSchema(t reflect.Type, components map[string]*schemaRef) *schemaRef {
	if t.Kind() == reflect.Pointer {
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
	// A generic instantiation's name (e.g. "ResourceList[github.com/app/models.User]")
	// embeds "/", "[", "]", and "." — characters that make the derived $ref an
	// invalid JSON pointer. Sanitize to a deterministic component key. Non-generic
	// names (no /[].) are returned unchanged, so existing keys never move.
	name = sanitizeComponentName(name)

	// Detect collision: if a schema with the same short name exists but from a
	// different package, disambiguate by appending the last segment of PkgPath.
	componentKey := name
	if existing, exists := components[name]; exists {
		// Check if it's truly the same type (same PkgPath) — if so, return ref.
		if existing.pkgPath == t.PkgPath() {
			return &schemaRef{Ref: "#/components/schemas/" + componentKey}
		}
		// Different package, same name — disambiguate with package suffix.
		pkg := t.PkgPath()
		if idx := strings.LastIndexByte(pkg, '/'); idx >= 0 {
			pkg = pkg[idx+1:]
		}
		componentKey = name + "_" + pkg
		// If even this disambiguated key exists and matches, return ref.
		if _, exists2 := components[componentKey]; exists2 {
			return &schemaRef{Ref: "#/components/schemas/" + componentKey}
		}
	}

	refPath := "#/components/schemas/" + componentKey

	schema := &schemaRef{
		Type:       "object",
		Properties: make(map[string]*schemaRef),
		pkgPath:    t.PkgPath(),
	}
	components[componentKey] = schema

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

// sanitizeComponentName turns a reflect type name into a valid OpenAPI
// component key (and thus a valid "#/components/schemas/<key>" JSON pointer).
//
// Non-generic names contain none of "/[]." and are returned unchanged, so they
// keep their existing keys. Generic instantiation names such as
// "ResourceList[github.com/app/models.User]" are reduced to "ResourceList_User"
// deterministically: the base type name, then each type argument trimmed to its
// final identifier (package path + qualifier dropped), joined by "_".
func sanitizeComponentName(name string) string {
	if !strings.ContainsAny(name, "/[].") {
		return name
	}

	base := name
	args := ""
	if i := strings.IndexByte(name, '['); i >= 0 {
		base = name[:i]
		args = strings.TrimSuffix(name[i+1:], "]")
	}

	var b strings.Builder
	b.WriteString(lastIdentSegment(base))
	if args != "" {
		for _, arg := range strings.Split(args, ",") {
			seg := lastIdentSegment(arg)
			if seg == "" {
				continue
			}
			b.WriteByte('_')
			b.WriteString(seg)
		}
	}
	return b.String()
}

// lastIdentSegment returns the final identifier of a (possibly qualified,
// possibly pointer/bracketed) type token: the substring after the last "/" or
// ".", with leading "*", "[", "]" stripped. e.g. "github.com/app/models.User"
// → "User", "*models.User" → "User".
func lastIdentSegment(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "*[]")
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		s = s[i+1:]
	}
	s = strings.Trim(s, "*[]")
	return s
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

		op := buildOperation(ri, components, app.config.problemJSON)

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

	if len(components) > 0 || len(app.config.openAPISecuritySchemes) > 0 {
		spec.Components = &openAPIComponents{
			Schemas:         components,
			SecuritySchemes: app.config.openAPISecuritySchemes,
		}
	}

	return app.config.JSONEncoder(spec)
}

// buildOperation creates an OpenAPI operation from route info.
func buildOperation(ri routeInfo, components map[string]*schemaRef, problemJSON bool) *openAPIOperation {
	op := &openAPIOperation{
		Description: ri.config.description,
		Tags:        ri.config.tags,
		Responses:   make(map[string]*openAPIResponse),
		Security:    ri.config.security,
	}

	if ri.resourceOp != nil {
		buildResourceOperation(op, ri.resourceOp, components, problemJSON)
		return op
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
					contentType: {Schema: bodySchema, Example: ri.config.requestExample},
				},
			}
		}
	}

	if outType != nil {
		outSchema := generateSchema(outType, components)
		op.Responses["200"] = &openAPIResponse{
			Description: "Successful response",
			Content: map[string]*openAPIMediaType{
				"application/json": {Schema: outSchema, Example: ri.config.responseExample},
			},
		}
	}

	if ri.hasValidate {
		contentType, schema := validationResponseSchema(components, problemJSON)
		op.Responses["422"] = &openAPIResponse{
			Description: "Validation failed",
			Content: map[string]*openAPIMediaType{
				contentType: {Schema: schema},
			},
		}
	}

	contentType, schema := defaultErrorResponseSchema(components, problemJSON)
	op.Responses["default"] = &openAPIResponse{
		Description: "Error response",
		Content: map[string]*openAPIMediaType{
			contentType: {Schema: schema},
		},
	}

	return op
}

// buildResourceOperation renders a kruda.Resource auto-CRUD operation
// explicitly from its resourceOp metadata, covering the list/get/create/update/
// delete shapes (200/201/204), a 422 only when validation is engaged, and the
// same default error response as typed routes.
func buildResourceOperation(op *openAPIOperation, rop *resourceOp, components map[string]*schemaRef, problemJSON bool) {
	if rop.idParam != "" {
		var idSchema *schemaRef
		if rop.idType != nil {
			idSchema = generateSchema(rop.idType, components)
		} else {
			idSchema = &schemaRef{Type: "string"}
		}
		op.Parameters = append(op.Parameters, openAPIParameter{
			Name:     rop.idParam,
			In:       "path",
			Required: true,
			Schema:   idSchema,
		})
	}

	if rop.needsListQuery {
		op.Parameters = append(op.Parameters,
			openAPIParameter{Name: "page", In: "query", Schema: &schemaRef{Type: "integer"}},
			openAPIParameter{Name: "limit", In: "query", Schema: &schemaRef{Type: "integer"}},
		)
	}

	if rop.bodyType != nil {
		op.RequestBody = &openAPIRequestBody{
			Required: true,
			Content: map[string]*openAPIMediaType{
				"application/json": {Schema: generateSchema(rop.bodyType, components)},
			},
		}
	}

	code := rop.successCode
	if code == "" {
		code = "200"
	}
	if rop.respType != nil {
		op.Responses[code] = &openAPIResponse{
			Description: "Successful response",
			Content: map[string]*openAPIMediaType{
				"application/json": {Schema: generateSchema(rop.respType, components)},
			},
		}
	} else {
		op.Responses[code] = &openAPIResponse{Description: "No Content"}
	}

	if rop.hasValidate {
		contentType, schema := validationResponseSchema(components, problemJSON)
		op.Responses["422"] = &openAPIResponse{
			Description: "Validation failed",
			Content: map[string]*openAPIMediaType{
				contentType: {Schema: schema},
			},
		}
	}

	contentType, schema := defaultErrorResponseSchema(components, problemJSON)
	op.Responses["default"] = &openAPIResponse{
		Description: "Error response",
		Content: map[string]*openAPIMediaType{
			contentType: {Schema: schema},
		},
	}
}

// validationResponseSchema returns the media type + schema for a 422 validation
// response. With problem+json it is a ProblemDetails; otherwise it is the
// ValidationError shape the validator actually marshals ({code, message, errors}).
func validationResponseSchema(components map[string]*schemaRef, problemJSON bool) (string, *schemaRef) {
	if problemJSON {
		ensureProblemDetailsSchema(components)
		return "application/problem+json", &schemaRef{Ref: "#/components/schemas/ProblemDetails"}
	}
	ensureValidationErrorSchema(components)
	return "application/json", &schemaRef{Ref: "#/components/schemas/ValidationError"}
}

// defaultErrorResponseSchema returns the media type + schema for a generic error
// response. With problem+json it is a ProblemDetails; otherwise it is a KrudaError.
func defaultErrorResponseSchema(components map[string]*schemaRef, problemJSON bool) (string, *schemaRef) {
	if problemJSON {
		ensureProblemDetailsSchema(components)
		return "application/problem+json", &schemaRef{Ref: "#/components/schemas/ProblemDetails"}
	}
	ensureKrudaErrorSchema(components)
	return "application/json", &schemaRef{Ref: "#/components/schemas/KrudaError"}
}

// ensureFieldErrorSchema documents FieldError. All five fields lack omitempty,
// so they are always present on the wire and are marked required.
func ensureFieldErrorSchema(components map[string]*schemaRef) {
	if _, ok := components["FieldError"]; ok {
		return
	}
	components["FieldError"] = &schemaRef{
		Type: "object",
		Properties: map[string]*schemaRef{
			"field":   {Type: "string"},
			"rule":    {Type: "string"},
			"param":   {Type: "string"},
			"message": {Type: "string"},
			"value":   {Type: "string"},
		},
		Required: []string{"field", "rule", "param", "message", "value"},
	}
}

func ensureProblemDetailsSchema(components map[string]*schemaRef) {
	if _, ok := components["ProblemDetails"]; ok {
		return
	}
	ensureFieldErrorSchema(components)
	components["ProblemDetails"] = &schemaRef{
		Type: "object",
		Properties: map[string]*schemaRef{
			"type":     {Type: "string"},
			"title":    {Type: "string"},
			"status":   {Type: "integer"},
			"detail":   {Type: "string"},
			"instance": {Type: "string"},
			"errors": {
				Type:  "array",
				Items: &schemaRef{Ref: "#/components/schemas/FieldError"},
			},
		},
		Required: []string{"type", "title", "status"},
	}
}

// ensureValidationErrorSchema documents the ValidationError wire shape the
// validator marshals: {code, message, errors[]} — all required.
func ensureValidationErrorSchema(components map[string]*schemaRef) {
	if _, ok := components["ValidationError"]; ok {
		return
	}
	ensureFieldErrorSchema(components)
	components["ValidationError"] = &schemaRef{
		Type: "object",
		Properties: map[string]*schemaRef{
			"code":    {Type: "integer"},
			"message": {Type: "string"},
			"errors": {
				Type:  "array",
				Items: &schemaRef{Ref: "#/components/schemas/FieldError"},
			},
		},
		Required: []string{"code", "message", "errors"},
	}
}

func ensureKrudaErrorSchema(components map[string]*schemaRef) {
	if _, ok := components["KrudaError"]; ok {
		return
	}
	components["KrudaError"] = &schemaRef{
		Type: "object",
		Properties: map[string]*schemaRef{
			"code":    {Type: "integer"},
			"message": {Type: "string"},
			"detail":  {Type: "string"},
		},
		Required: []string{"code", "message"},
	}
}
