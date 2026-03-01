package kruda

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"reflect"
	"testing"
)

type bindMockRequest struct {
	mockRequest
	queryParams map[string]string
}

func (r *bindMockRequest) QueryParam(key string) string {
	if r.queryParams != nil {
		return r.queryParams[key]
	}
	return ""
}

// bindCtx creates a Ctx wired to an App with the given method, path, params, query, and body.
func bindCtx(method, path string, params map[string]string, query map[string]string, body []byte) *Ctx {
	app := New()
	req := &bindMockRequest{
		mockRequest: mockRequest{method: method, path: path, body: body},
		queryParams: query,
	}
	resp := newMockResponse()
	c := newCtx(app)
	c.reset(resp, req)
	c.method = method
	c.path = path
	for k, v := range params {
		c.params.set(k, v)
	}
	return c
}

func TestSelectConverter_String(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(""))
	v, err := conv("hello")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "hello" {
		t.Errorf("got %q, want hello", v.String())
	}
}

func TestSelectConverter_Int(t *testing.T) {
	for _, tt := range []struct {
		typ  reflect.Type
		in   string
		want int64
	}{
		{reflect.TypeOf(int(0)), "42", 42},
		{reflect.TypeOf(int8(0)), "127", 127},
		{reflect.TypeOf(int16(0)), "1000", 1000},
		{reflect.TypeOf(int32(0)), "100000", 100000},
		{reflect.TypeOf(int64(0)), "9999999", 9999999},
	} {
		conv := selectConverter(tt.typ)
		v, err := conv(tt.in)
		if err != nil {
			t.Fatalf("type %v: %v", tt.typ, err)
		}
		if v.Int() != tt.want {
			t.Errorf("type %v: got %d, want %d", tt.typ, v.Int(), tt.want)
		}
	}
}

func TestSelectConverter_Uint(t *testing.T) {
	for _, tt := range []struct {
		typ  reflect.Type
		in   string
		want uint64
	}{
		{reflect.TypeOf(uint(0)), "42", 42},
		{reflect.TypeOf(uint8(0)), "255", 255},
		{reflect.TypeOf(uint16(0)), "1000", 1000},
		{reflect.TypeOf(uint32(0)), "100000", 100000},
		{reflect.TypeOf(uint64(0)), "9999999", 9999999},
	} {
		conv := selectConverter(tt.typ)
		v, err := conv(tt.in)
		if err != nil {
			t.Fatalf("type %v: %v", tt.typ, err)
		}
		if v.Uint() != tt.want {
			t.Errorf("type %v: got %d, want %d", tt.typ, v.Uint(), tt.want)
		}
	}
}

func TestSelectConverter_Float(t *testing.T) {
	for _, tt := range []struct {
		typ  reflect.Type
		in   string
		want float64
	}{
		{reflect.TypeOf(float32(0)), "3.14", 3.14},
		{reflect.TypeOf(float64(0)), "2.718", 2.718},
	} {
		conv := selectConverter(tt.typ)
		v, err := conv(tt.in)
		if err != nil {
			t.Fatalf("type %v: %v", tt.typ, err)
		}
		// float32 loses precision, so compare with tolerance
		if diff := v.Float() - tt.want; diff > 0.01 || diff < -0.01 {
			t.Errorf("type %v: got %f, want %f", tt.typ, v.Float(), tt.want)
		}
	}
}

func TestSelectConverter_Bool(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(false))
	v, err := conv("true")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Bool() {
		t.Error("expected true")
	}
	v, err = conv("false")
	if err != nil {
		t.Fatal(err)
	}
	if v.Bool() {
		t.Error("expected false")
	}
}

func TestSelectConverter_InvalidInput(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(int(0)))
	_, err := conv("not-a-number")
	if err == nil {
		t.Error("expected error for invalid int")
	}

	conv = selectConverter(reflect.TypeOf(false))
	_, err = conv("not-a-bool")
	if err == nil {
		t.Error("expected error for invalid bool")
	}
}

func TestSelectConverter_UnsupportedTypePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unsupported type")
		}
	}()
	selectConverter(reflect.TypeOf(struct{}{}))
}

func TestBuildInputParser_MixedTags(t *testing.T) {
	type Input struct {
		ID   string `param:"id"`
		Page int    `query:"page" default:"1"`
		Name string `json:"name"`
	}
	p := buildInputParser[Input]()
	if len(p.paramFields) != 1 {
		t.Errorf("paramFields = %d, want 1", len(p.paramFields))
	}
	if len(p.queryFields) != 1 {
		t.Errorf("queryFields = %d, want 1", len(p.queryFields))
	}
	if !p.hasBody {
		t.Error("hasBody should be true")
	}
	if len(p.defaults) != 1 {
		t.Errorf("defaults = %d, want 1", len(p.defaults))
	}
	// Default value should be pre-parsed as int 1
	if p.defaults[0].value.Int() != 1 {
		t.Errorf("default value = %v, want 1", p.defaults[0].value)
	}
}

func TestBuildInputParser_UnexportedFieldsSkipped(t *testing.T) {
	type Input struct {
		Name   string `json:"name"`
		secret string //nolint:unused
	}
	p := buildInputParser[Input]()
	// Only exported Name should be detected
	if !p.hasBody {
		t.Error("hasBody should be true for Name")
	}
	// No param/query fields
	if len(p.paramFields) != 0 || len(p.queryFields) != 0 {
		t.Error("unexported fields should be skipped")
	}
}

func TestBuildInputParser_NonStructPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-struct type")
		}
	}()
	buildInputParser[string]()
}

func TestBuildInputParser_InvalidDefaultPanics(t *testing.T) {
	type Input struct {
		Count int `query:"count" default:"not-a-number"`
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid default")
		}
	}()
	buildInputParser[Input]()
}

func TestBuildInputParser_NoTags(t *testing.T) {
	type Input struct {
		Name string // no tags at all
	}
	p := buildInputParser[Input]()
	if p.hasBody {
		t.Error("hasBody should be false")
	}
	if len(p.paramFields) != 0 || len(p.queryFields) != 0 || len(p.defaults) != 0 {
		t.Error("no fields should be parsed")
	}
}

func TestParse_BodyOnly(t *testing.T) {
	type Input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"John","email":"john@example.com"}`))

	val, err := p.parse(c)
	if err != nil {
		t.Fatal(err)
	}
	in := val.Interface().(Input)
	if in.Name != "John" {
		t.Errorf("Name = %q, want John", in.Name)
	}
	if in.Email != "john@example.com" {
		t.Errorf("Email = %q, want john@example.com", in.Email)
	}
}

func TestParse_ParamOnly(t *testing.T) {
	type Input struct {
		ID string `param:"id"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("GET", "/users/abc", map[string]string{"id": "abc"}, nil, nil)

	val, err := p.parse(c)
	if err != nil {
		t.Fatal(err)
	}
	in := val.Interface().(Input)
	if in.ID != "abc" {
		t.Errorf("ID = %q, want abc", in.ID)
	}
}

func TestParse_QueryOnly(t *testing.T) {
	type Input struct {
		Page int    `query:"page"`
		Sort string `query:"sort"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("GET", "/items", nil, map[string]string{"page": "2", "sort": "name"}, nil)

	val, err := p.parse(c)
	if err != nil {
		t.Fatal(err)
	}
	in := val.Interface().(Input)
	if in.Page != 2 {
		t.Errorf("Page = %d, want 2", in.Page)
	}
	if in.Sort != "name" {
		t.Errorf("Sort = %q, want name", in.Sort)
	}
}

func TestParse_DefaultsApplied(t *testing.T) {
	type Input struct {
		Page int    `query:"page" default:"1"`
		Sort string `query:"sort" default:"created_at"`
	}
	p := buildInputParser[Input]()
	// No query params provided — defaults should be used
	c := bindCtx("GET", "/items", nil, nil, nil)

	val, err := p.parse(c)
	if err != nil {
		t.Fatal(err)
	}
	in := val.Interface().(Input)
	if in.Page != 1 {
		t.Errorf("Page = %d, want 1 (default)", in.Page)
	}
	if in.Sort != "created_at" {
		t.Errorf("Sort = %q, want created_at (default)", in.Sort)
	}
}

func TestParse_QueryOverwritesDefault(t *testing.T) {
	type Input struct {
		Page int `query:"page" default:"1"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("GET", "/items", nil, map[string]string{"page": "5"}, nil)

	val, err := p.parse(c)
	if err != nil {
		t.Fatal(err)
	}
	in := val.Interface().(Input)
	if in.Page != 5 {
		t.Errorf("Page = %d, want 5 (query overwrites default)", in.Page)
	}
}

func TestParse_ParamOverwritesBody(t *testing.T) {
	type Input struct {
		ID   string `param:"id" json:"id"`
		Name string `json:"name"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("POST", "/users/from-path", map[string]string{"id": "from-path"}, nil,
		[]byte(`{"id":"from-body","name":"John"}`))

	val, err := p.parse(c)
	if err != nil {
		t.Fatal(err)
	}
	in := val.Interface().(Input)
	if in.ID != "from-path" {
		t.Errorf("ID = %q, want from-path (param overwrites body)", in.ID)
	}
	if in.Name != "John" {
		t.Errorf("Name = %q, want John", in.Name)
	}
}

func TestParse_MixedSources(t *testing.T) {
	type Input struct {
		ID   string `param:"id"`
		Page int    `query:"page" default:"1"`
		Name string `json:"name"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("POST", "/users/u123", map[string]string{"id": "u123"},
		map[string]string{"page": "3"}, []byte(`{"name":"Alice"}`))

	val, err := p.parse(c)
	if err != nil {
		t.Fatal(err)
	}
	in := val.Interface().(Input)
	if in.ID != "u123" {
		t.Errorf("ID = %q, want u123", in.ID)
	}
	if in.Page != 3 {
		t.Errorf("Page = %d, want 3", in.Page)
	}
	if in.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", in.Name)
	}
}

func TestParse_GETSkipsBody(t *testing.T) {
	type Input struct {
		Name string `json:"name"`
		Page int    `query:"page" default:"1"`
	}
	p := buildInputParser[Input]()
	// GET with body — body should be ignored
	c := bindCtx("GET", "/items", nil, nil, []byte(`{"name":"ignored"}`))

	val, err := p.parse(c)
	if err != nil {
		t.Fatal(err)
	}
	in := val.Interface().(Input)
	if in.Name != "" {
		t.Errorf("Name = %q, want empty (GET skips body)", in.Name)
	}
	if in.Page != 1 {
		t.Errorf("Page = %d, want 1 (default)", in.Page)
	}
}

func TestParse_EmptyBodyError(t *testing.T) {
	type Input struct {
		Name string `json:"name"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("POST", "/users", nil, nil, []byte{})

	_, err := p.parse(c)
	if err == nil {
		t.Fatal("expected error for empty body on POST")
	}
}

func TestParse_InvalidBodyError(t *testing.T) {
	type Input struct {
		Name string `json:"name"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("POST", "/users", nil, nil, []byte(`{invalid json`))

	_, err := p.parse(c)
	if err == nil {
		t.Fatal("expected error for invalid JSON body")
	}
}

func TestParse_InvalidQueryTypeError(t *testing.T) {
	type Input struct {
		Page int `query:"page"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("GET", "/items", nil, map[string]string{"page": "not-a-number"}, nil)

	_, err := p.parse(c)
	if err == nil {
		t.Fatal("expected error for invalid query type")
	}
}

func TestParse_InvalidParamTypeError(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	p := buildInputParser[Input]()
	c := bindCtx("GET", "/users/abc", map[string]string{"id": "abc"}, nil, nil)

	_, err := p.parse(c)
	if err == nil {
		t.Fatal("expected error for invalid param type")
	}
}

func TestHasBody(t *testing.T) {
	if !hasBody("POST") {
		t.Error("POST should have body")
	}
	if !hasBody("PUT") {
		t.Error("PUT should have body")
	}
	if !hasBody("PATCH") {
		t.Error("PATCH should have body")
	}
	if hasBody("GET") {
		t.Error("GET should not have body")
	}
	if hasBody("DELETE") {
		t.Error("DELETE should not have body")
	}
	if hasBody("HEAD") {
		t.Error("HEAD should not have body")
	}
}

// multipartMockRequest wraps a real *http.Request so RawRequest() returns it.
type multipartMockRequest struct {
	mockRequest
	raw *http.Request
}

func (r *multipartMockRequest) RawRequest() any { return r.raw }

func (r *multipartMockRequest) MultipartForm(maxBytes int64) (*multipart.Form, error) {
	if err := r.raw.ParseMultipartForm(maxBytes); err != nil {
		return nil, err
	}
	return r.raw.MultipartForm, nil
}

// createMultipartRequest builds a real *http.Request with multipart form data.
func createMultipartRequest(t *testing.T, fields map[string]string, files map[string][]fileEntry) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Add text fields
	for name, value := range fields {
		if err := w.WriteField(name, value); err != nil {
			t.Fatalf("write field %q: %v", name, err)
		}
	}

	// Add file fields
	for name, entries := range files {
		for _, fe := range entries {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, name, fe.filename))
			h.Set("Content-Type", fe.contentType)
			part, err := w.CreatePart(h)
			if err != nil {
				t.Fatalf("create part %q: %v", name, err)
			}
			if _, err := part.Write(fe.content); err != nil {
				t.Fatalf("write part %q: %v", name, err)
			}
		}
	}
	w.Close()

	req, err := http.NewRequest("POST", "/upload", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

type fileEntry struct {
	filename    string
	contentType string
	content     []byte
}

// multipartCtx creates a Ctx wired to a real *http.Request with multipart data.
func multipartCtx(t *testing.T, fields map[string]string, files map[string][]fileEntry, params map[string]string, query map[string]string) *Ctx {
	t.Helper()
	httpReq := createMultipartRequest(t, fields, files)
	// Add query params to URL
	if len(query) > 0 {
		q := httpReq.URL.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		httpReq.URL.RawQuery = q.Encode()
	}

	app := New()
	req := &multipartMockRequest{
		mockRequest: mockRequest{method: "POST", path: "/upload", query: query},
		raw:         httpReq,
	}
	resp := newMockResponse()
	c := newCtx(app)
	c.reset(resp, req)
	c.method = "POST"
	c.path = "/upload"
	for k, v := range params {
		c.params.set(k, v)
	}
	return c
}

func TestBuildInputParser_FormTagDetection(t *testing.T) {
	type Input struct {
		Avatar *FileUpload   `form:"avatar"`
		Photos []*FileUpload `form:"photos"`
		Name   string        `form:"name"`
	}
	p := buildInputParser[Input]()

	if !p.hasForm {
		t.Error("hasForm should be true")
	}
	if len(p.formFields) != 3 {
		t.Fatalf("formFields = %d, want 3", len(p.formFields))
	}

	// Check field categorization
	avatar := p.formFields[0]
	if !avatar.isFile || avatar.isMulti || avatar.isString {
		t.Errorf("avatar: isFile=%v isMulti=%v isString=%v, want isFile=true", avatar.isFile, avatar.isMulti, avatar.isString)
	}
	if avatar.tag != "avatar" {
		t.Errorf("avatar tag = %q, want avatar", avatar.tag)
	}

	photos := p.formFields[1]
	if photos.isFile || !photos.isMulti || photos.isString {
		t.Errorf("photos: isFile=%v isMulti=%v isString=%v, want isMulti=true", photos.isFile, photos.isMulti, photos.isString)
	}

	name := p.formFields[2]
	if name.isFile || name.isMulti || !name.isString {
		t.Errorf("name: isFile=%v isMulti=%v isString=%v, want isString=true", name.isFile, name.isMulti, name.isString)
	}
}

func TestBuildInputParser_FormUnsupportedTypePanics(t *testing.T) {
	type Input struct {
		Count int `form:"count"`
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unsupported form field type int")
		}
	}()
	buildInputParser[Input]()
}

func TestBuildInputParser_FormMutuallyExclusiveWithBody(t *testing.T) {
	// A struct with form tags should set hasForm=true
	// hasBody can also be true if json tags exist, but parse() will branch on hasForm first
	type Input struct {
		Avatar *FileUpload `form:"avatar"`
		Name   string      `json:"name"`
	}
	p := buildInputParser[Input]()
	if !p.hasForm {
		t.Error("hasForm should be true")
	}
	if !p.hasBody {
		t.Error("hasBody should also be true (json tag exists)")
	}
	// The key behavior: parse() will use form path, not JSON body path
}

func TestParse_MultipartSingleFile(t *testing.T) {
	type Input struct {
		Avatar *FileUpload `form:"avatar"`
		Name   string      `form:"name"`
	}
	p := buildInputParser[Input]()

	c := multipartCtx(t,
		map[string]string{"name": "John"},
		map[string][]fileEntry{
			"avatar": {{filename: "photo.png", contentType: "image/png", content: []byte("PNG data")}},
		},
		nil, nil,
	)

	val, err := p.parse(c)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	in := val.Interface().(Input)

	if in.Name != "John" {
		t.Errorf("Name = %q, want John", in.Name)
	}
	if in.Avatar == nil {
		t.Fatal("Avatar should not be nil")
	}
	if in.Avatar.Name != "photo.png" {
		t.Errorf("Avatar.Name = %q, want photo.png", in.Avatar.Name)
	}
	if in.Avatar.ContentType != "image/png" {
		t.Errorf("Avatar.ContentType = %q, want image/png", in.Avatar.ContentType)
	}
	if in.Avatar.Size != int64(len("PNG data")) {
		t.Errorf("Avatar.Size = %d, want %d", in.Avatar.Size, len("PNG data"))
	}
}

func TestParse_MultipartMultipleFiles(t *testing.T) {
	type Input struct {
		Photos []*FileUpload `form:"photos"`
	}
	p := buildInputParser[Input]()

	c := multipartCtx(t,
		nil,
		map[string][]fileEntry{
			"photos": {
				{filename: "a.jpg", contentType: "image/jpeg", content: []byte("JPEG1")},
				{filename: "b.png", contentType: "image/png", content: []byte("PNG2")},
			},
		},
		nil, nil,
	)

	val, err := p.parse(c)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	in := val.Interface().(Input)

	if len(in.Photos) != 2 {
		t.Fatalf("Photos = %d, want 2", len(in.Photos))
	}
	if in.Photos[0].Name != "a.jpg" {
		t.Errorf("Photos[0].Name = %q, want a.jpg", in.Photos[0].Name)
	}
	if in.Photos[1].Name != "b.png" {
		t.Errorf("Photos[1].Name = %q, want b.png", in.Photos[1].Name)
	}
}

func TestParse_MultipartTextOnly(t *testing.T) {
	type Input struct {
		Name  string `form:"name"`
		Email string `form:"email"`
	}
	p := buildInputParser[Input]()

	c := multipartCtx(t,
		map[string]string{"name": "Alice", "email": "alice@example.com"},
		nil, nil, nil,
	)

	val, err := p.parse(c)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	in := val.Interface().(Input)

	if in.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", in.Name)
	}
	if in.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", in.Email)
	}
}

func TestParse_MultipartMissingFileIsNil(t *testing.T) {
	type Input struct {
		Avatar *FileUpload `form:"avatar"`
	}
	p := buildInputParser[Input]()

	// No files in the multipart request
	c := multipartCtx(t, nil, nil, nil, nil)

	val, err := p.parse(c)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	in := val.Interface().(Input)

	if in.Avatar != nil {
		t.Error("Avatar should be nil when no file uploaded")
	}
}

func TestParse_MultipartWithQueryAndParam(t *testing.T) {
	type Input struct {
		ID   string      `param:"id"`
		Page int         `query:"page" default:"1"`
		Name string      `form:"name"`
		File *FileUpload `form:"file"`
	}
	p := buildInputParser[Input]()

	c := multipartCtx(t,
		map[string]string{"name": "Bob"},
		map[string][]fileEntry{
			"file": {{filename: "doc.pdf", contentType: "application/pdf", content: []byte("PDF")}},
		},
		map[string]string{"id": "u42"},
		map[string]string{"page": "3"},
	)

	val, err := p.parse(c)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	in := val.Interface().(Input)

	if in.ID != "u42" {
		t.Errorf("ID = %q, want u42", in.ID)
	}
	if in.Page != 3 {
		t.Errorf("Page = %d, want 3", in.Page)
	}
	if in.Name != "Bob" {
		t.Errorf("Name = %q, want Bob", in.Name)
	}
	if in.File == nil {
		t.Fatal("File should not be nil")
	}
	if in.File.Name != "doc.pdf" {
		t.Errorf("File.Name = %q, want doc.pdf", in.File.Name)
	}
}

func TestParse_MultipartNonHTTPRequestError(t *testing.T) {
	type Input struct {
		Name string `form:"name"`
	}
	p := buildInputParser[Input]()

	// Use a regular mock that returns nil from RawRequest()
	c := bindCtx("POST", "/upload", nil, nil, nil)

	_, err := p.parse(c)
	if err == nil {
		t.Fatal("expected error for non-http.Request transport")
	}
}

func TestParse_MultipartWithDefaults(t *testing.T) {
	type Input struct {
		Name    string `form:"name"`
		Country string `form:"country" default:"TH"`
	}
	// Note: default tag on a form string field — default is set first, then form value overwrites
	// But since "country" is a form field with default, and the form doesn't provide it,
	// the default should remain. However, default uses selectConverter which expects
	// the field to have a param/query tag. Let me check...
	// Actually, looking at buildInputParser, default tag calls selectConverter on the field type.
	// For string fields with form tag, selectConverter(string) works fine.
	p := buildInputParser[Input]()

	// Only provide "name", not "country"
	c := multipartCtx(t,
		map[string]string{"name": "Test"},
		nil, nil, nil,
	)

	val, err := p.parse(c)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	in := val.Interface().(Input)

	if in.Name != "Test" {
		t.Errorf("Name = %q, want Test", in.Name)
	}
	if in.Country != "TH" {
		t.Errorf("Country = %q, want TH (default)", in.Country)
	}
}
