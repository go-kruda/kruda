package kruda

// Validates: Requirements 1.1, 8.1
//
// Property: For any value of a supported Go type, selectConverter round-trips correctly.
// That is, converting a value to its string representation and back via the converter
// produces the original value.

import (
	"bytes"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"reflect"
	"strconv"
	"testing"
	"testing/quick"
)

// TestPropertySelectConverterStringRoundTrip checks that for any string s,
// the string converter returns s unchanged.
func TestPropertySelectConverterStringRoundTrip(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(""))
	f := func(s string) bool {
		val, err := conv(s)
		if err != nil {
			return false
		}
		return val.String() == s
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("string converter round-trip failed: %v", err)
	}
}

// TestPropertySelectConverterInt64RoundTrip checks that for any int64 n,
// converting to string via FormatInt and back via the converter produces n.
func TestPropertySelectConverterInt64RoundTrip(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(int64(0)))
	f := func(n int64) bool {
		s := strconv.FormatInt(n, 10)
		val, err := conv(s)
		if err != nil {
			return false
		}
		return val.Int() == n
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("int64 converter round-trip failed: %v", err)
	}
}

// TestPropertySelectConverterIntRoundTrip checks that for any int n,
// converting to string and back via the converter produces n.
func TestPropertySelectConverterIntRoundTrip(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(int(0)))
	f := func(n int) bool {
		s := strconv.FormatInt(int64(n), 10)
		val, err := conv(s)
		if err != nil {
			return false
		}
		return val.Convert(reflect.TypeOf(int(0))).Interface().(int) == n
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("int converter round-trip failed: %v", err)
	}
}

// TestPropertySelectConverterUint64RoundTrip checks that for any uint64 n,
// converting to string via FormatUint and back via the converter produces n.
func TestPropertySelectConverterUint64RoundTrip(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(uint64(0)))
	f := func(n uint64) bool {
		s := strconv.FormatUint(n, 10)
		val, err := conv(s)
		if err != nil {
			return false
		}
		return val.Uint() == n
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("uint64 converter round-trip failed: %v", err)
	}
}

// TestPropertySelectConverterFloat64RoundTrip checks that for any finite float64 f,
// converting to string via FormatFloat('f', -1, 64) and back produces f.
// NaN and Inf are filtered out since they don't round-trip with string formatting.
func TestPropertySelectConverterFloat64RoundTrip(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(float64(0)))
	f := func(n float64) bool {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return true // skip non-finite values
		}
		s := strconv.FormatFloat(n, 'f', -1, 64)
		val, err := conv(s)
		if err != nil {
			return false
		}
		return val.Float() == n
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("float64 converter round-trip failed: %v", err)
	}
}

// TestPropertySelectConverterFloat32RoundTrip checks that for any finite float32 f,
// converting to string via FormatFloat('f', -1, 32) and back produces f.
func TestPropertySelectConverterFloat32RoundTrip(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(float32(0)))
	f := func(n float32) bool {
		if math.IsNaN(float64(n)) || math.IsInf(float64(n), 0) {
			return true // skip non-finite values
		}
		s := strconv.FormatFloat(float64(n), 'f', -1, 32)
		val, err := conv(s)
		if err != nil {
			return false
		}
		got := float32(val.Float())
		return got == n
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("float32 converter round-trip failed: %v", err)
	}
}

// TestPropertySelectConverterBoolRoundTrip checks that for any bool b,
// converting to string via FormatBool and back via the converter produces b.
func TestPropertySelectConverterBoolRoundTrip(t *testing.T) {
	conv := selectConverter(reflect.TypeOf(false))
	f := func(b bool) bool {
		s := strconv.FormatBool(b)
		val, err := conv(s)
		if err != nil {
			return false
		}
		return val.Bool() == b
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("bool converter round-trip failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase2b-extensions, Property 3: File Upload Binding Populates FileUpload Correctly
//
// For any multipart form request containing files with arbitrary names, sizes,
// and content types, the binding pipeline should populate *FileUpload fields
// with matching Name, Size, and ContentType values, and []*FileUpload fields
// should contain all uploaded files for that field name.
//
// **Validates: R4.2, R4.3**
// ---------------------------------------------------------------------------

// safeFilename generates a safe filename from random bytes (alphanumeric + extension).
func safeFilename(seed uint8) string {
	names := []string{
		"photo.png", "doc.pdf", "image.jpg", "file.txt", "data.csv",
		"report.xlsx", "video.mp4", "audio.mp3", "archive.zip", "readme.md",
		"test.go", "main.rs", "app.js", "style.css", "index.html",
		"config.yaml",
	}
	return names[int(seed)%len(names)]
}

// safeContentType picks a valid MIME type from a seed.
func safeContentType(seed uint8) string {
	types := []string{
		"image/png", "image/jpeg", "application/pdf", "text/plain",
		"text/csv", "application/json", "video/mp4", "audio/mpeg",
		"application/zip", "text/html", "application/xml", "image/gif",
	}
	return types[int(seed)%len(types)]
}

func TestPropertyFileUploadBindingPopulatesCorrectly(t *testing.T) {
	type SingleFileInput struct {
		Avatar *FileUpload `form:"avatar"`
	}

	f := func(filenameSeed, ctSeed uint8, contentLen uint8) bool {
		filename := safeFilename(filenameSeed)
		contentType := safeContentType(ctSeed)
		// Generate content of length 1..255 (avoid 0 which creates empty file)
		size := int(contentLen) + 1
		content := make([]byte, size)
		for i := range content {
			content[i] = byte('A' + (i % 26))
		}

		p := buildInputParser[SingleFileInput]()

		// Build multipart request
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="avatar"; filename="`+filename+`"`)
		h.Set("Content-Type", contentType)
		part, err := w.CreatePart(h)
		if err != nil {
			return false
		}
		if _, err := part.Write(content); err != nil {
			return false
		}
		w.Close()

		req, err := http.NewRequest("POST", "/upload", &buf)
		if err != nil {
			return false
		}
		req.Header.Set("Content-Type", w.FormDataContentType())

		app := New()
		mockReq := &multipartMockRequest{
			mockRequest: mockRequest{method: "POST", path: "/upload"},
			raw:         req,
		}
		resp := newMockResponse()
		c := newCtx(app)
		c.reset(resp, mockReq)
		c.method = "POST"
		c.path = "/upload"

		val, err := p.parse(c)
		if err != nil {
			return false
		}
		in := val.Interface().(SingleFileInput)

		if in.Avatar == nil {
			return false
		}
		if in.Avatar.Name != filename {
			return false
		}
		if in.Avatar.Size != int64(size) {
			return false
		}
		if in.Avatar.ContentType != contentType {
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 3 failed: %v", err)
	}
}

func TestPropertyMultiFileUploadBindingPopulatesAll(t *testing.T) {
	type MultiFileInput struct {
		Photos []*FileUpload `form:"photos"`
	}

	f := func(count uint8) bool {
		// 1..5 files
		n := int(count%5) + 1

		p := buildInputParser[MultiFileInput]()

		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		type expected struct {
			name string
			ct   string
			size int
		}
		var want []expected

		for i := 0; i < n; i++ {
			fname := safeFilename(uint8(i * 3))
			ct := safeContentType(uint8(i * 7))
			content := make([]byte, i+1)
			for j := range content {
				content[j] = byte('a' + (j % 26))
			}
			want = append(want, expected{name: fname, ct: ct, size: len(content)})

			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", `form-data; name="photos"; filename="`+fname+`"`)
			h.Set("Content-Type", ct)
			part, err := w.CreatePart(h)
			if err != nil {
				return false
			}
			if _, err := part.Write(content); err != nil {
				return false
			}
		}
		w.Close()

		req, err := http.NewRequest("POST", "/upload", &buf)
		if err != nil {
			return false
		}
		req.Header.Set("Content-Type", w.FormDataContentType())

		app := New()
		mockReq := &multipartMockRequest{
			mockRequest: mockRequest{method: "POST", path: "/upload"},
			raw:         req,
		}
		resp := newMockResponse()
		c := newCtx(app)
		c.reset(resp, mockReq)
		c.method = "POST"
		c.path = "/upload"

		val, err := p.parse(c)
		if err != nil {
			return false
		}
		in := val.Interface().(MultiFileInput)

		if len(in.Photos) != n {
			return false
		}
		for i, photo := range in.Photos {
			if photo.Name != want[i].name {
				return false
			}
			if photo.Size != int64(want[i].size) {
				return false
			}
			if photo.ContentType != want[i].ct {
				return false
			}
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 3 (multi-file) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase2b-extensions, Property 4: Form Text Field Binding
//
// For any multipart form request containing text fields with arbitrary string
// values, the binding pipeline should populate string fields tagged with `form`
// with the exact submitted value.
//
// **Validates: R4.4**
// ---------------------------------------------------------------------------

func TestPropertyFormTextFieldBinding(t *testing.T) {
	type TextInput struct {
		Name  string `form:"name"`
		Email string `form:"email"`
	}

	f := func(name, email string) bool {
		// Skip empty strings — form binding skips empty values by design
		if name == "" || email == "" {
			return true
		}
		// Skip strings with null bytes — multipart encoding doesn't handle them
		for _, c := range name {
			if c == 0 {
				return true
			}
		}
		for _, c := range email {
			if c == 0 {
				return true
			}
		}

		p := buildInputParser[TextInput]()

		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		if err := w.WriteField("name", name); err != nil {
			return false
		}
		if err := w.WriteField("email", email); err != nil {
			return false
		}
		w.Close()

		req, err := http.NewRequest("POST", "/form", &buf)
		if err != nil {
			return false
		}
		req.Header.Set("Content-Type", w.FormDataContentType())

		app := New()
		mockReq := &multipartMockRequest{
			mockRequest: mockRequest{method: "POST", path: "/form"},
			raw:         req,
		}
		resp := newMockResponse()
		c := newCtx(app)
		c.reset(resp, mockReq)
		c.method = "POST"
		c.path = "/form"

		val, err := p.parse(c)
		if err != nil {
			return false
		}
		in := val.Interface().(TextInput)

		return in.Name == name && in.Email == email
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 4 failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase2b-extensions, Property 5: Form Binding Priority Order
//
// For any struct with both form and param/query tagged fields, when the same
// logical value is provided from multiple sources, path params should take
// highest priority, then query params, then form values — matching the
// pipeline order: defaults → form → query → param.
//
// **Validates: R4.8**
// ---------------------------------------------------------------------------

func TestPropertyFormBindingPriorityOrder(t *testing.T) {
	// This struct has a "name" field bound to form, query, AND param.
	// We test that param > query > form > default.
	type PriorityInput struct {
		Name string `form:"name" query:"name" param:"name" default:"default_val"`
	}

	f := func(formVal, queryVal, paramVal uint8) bool {
		// Generate distinct non-empty values from seeds
		formStr := "form_" + strconv.Itoa(int(formVal))
		queryStr := "query_" + strconv.Itoa(int(queryVal))
		paramStr := "param_" + strconv.Itoa(int(paramVal))

		p := buildInputParser[PriorityInput]()

		// Build multipart with form value
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("name", formStr)
		w.Close()

		req, _ := http.NewRequest("POST", "/test", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		// Add query param
		q := req.URL.Query()
		q.Set("name", queryStr)
		req.URL.RawQuery = q.Encode()

		app := New()
		mockReq := &multipartMockRequest{
			mockRequest: mockRequest{method: "POST", path: "/test", query: map[string]string{"name": queryStr}},
			raw:         req,
		}
		resp := newMockResponse()
		c := newCtx(app)
		c.reset(resp, mockReq)
		c.method = "POST"
		c.path = "/test"
		c.params["name"] = paramStr

		val, err := p.parse(c)
		if err != nil {
			return false
		}
		in := val.Interface().(PriorityInput)

		// Param has highest priority — should always win
		return in.Name == paramStr
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 5 (param wins) failed: %v", err)
	}

	// Test that query > form when no param is provided
	fQueryWins := func(formVal, queryVal uint8) bool {
		formStr := "form_" + strconv.Itoa(int(formVal))
		queryStr := "query_" + strconv.Itoa(int(queryVal))

		type QueryWinsInput struct {
			Name string `form:"name" query:"name" default:"default_val"`
		}
		p := buildInputParser[QueryWinsInput]()

		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("name", formStr)
		w.Close()

		req, _ := http.NewRequest("POST", "/test", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())

		app := New()
		mockReq := &multipartMockRequest{
			mockRequest: mockRequest{method: "POST", path: "/test", query: map[string]string{"name": queryStr}},
			raw:         req,
		}
		resp := newMockResponse()
		c := newCtx(app)
		c.reset(resp, mockReq)
		c.method = "POST"
		c.path = "/test"

		val, err := p.parse(c)
		if err != nil {
			return false
		}
		in := val.Interface().(QueryWinsInput)

		// Query overwrites form
		return in.Name == queryStr
	}

	if err := quick.Check(fQueryWins, cfg); err != nil {
		t.Errorf("Property 5 (query wins over form) failed: %v", err)
	}

	// Test that form > default when no query/param
	fFormWins := func(formVal uint8) bool {
		formStr := "form_" + strconv.Itoa(int(formVal))

		type FormWinsInput struct {
			Name string `form:"name" default:"default_val"`
		}
		p := buildInputParser[FormWinsInput]()

		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("name", formStr)
		w.Close()

		req, _ := http.NewRequest("POST", "/test", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())

		app := New()
		mockReq := &multipartMockRequest{
			mockRequest: mockRequest{method: "POST", path: "/test"},
			raw:         req,
		}
		resp := newMockResponse()
		c := newCtx(app)
		c.reset(resp, mockReq)
		c.method = "POST"
		c.path = "/test"

		val, err := p.parse(c)
		if err != nil {
			return false
		}
		in := val.Interface().(FormWinsInput)

		return in.Name == formStr
	}

	if err := quick.Check(fFormWins, cfg); err != nil {
		t.Errorf("Property 5 (form wins over default) failed: %v", err)
	}
}
