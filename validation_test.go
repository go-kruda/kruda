package kruda

import (
	"encoding/json"
	"reflect"
	"testing"
)

// valueOf is a helper that returns reflect.Value of the underlying value.
func valueOf(v any) reflect.Value {
	return reflect.ValueOf(v)
}

// ---------------------------------------------------------------------------
// Task 13.1: ValidationError.Error() and MarshalJSON
// ---------------------------------------------------------------------------

func TestValidationError_Error_Single(t *testing.T) {
	ve := &ValidationError{Errors: []FieldError{
		{Field: "email", Rule: "required", Message: "email is required"},
	}}
	if got := ve.Error(); got != "email is required" {
		t.Errorf("Error() = %q, want %q", got, "email is required")
	}
}

func TestValidationError_Error_Multiple(t *testing.T) {
	ve := &ValidationError{Errors: []FieldError{
		{Field: "email", Rule: "required", Message: "email is required"},
		{Field: "name", Rule: "min", Param: "2", Message: "name must be at least 2"},
	}}
	if got := ve.Error(); got != "validation failed: 2 errors" {
		t.Errorf("Error() = %q, want %q", got, "validation failed: 2 errors")
	}
}

func TestValidationError_MarshalJSON(t *testing.T) {
	ve := &ValidationError{Errors: []FieldError{
		{Field: "email", Rule: "required", Param: "", Message: "email is required", Value: ""},
		{Field: "age", Rule: "min", Param: "18", Message: "age must be at least 18", Value: "15"},
	}}
	data, err := ve.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["code"] != float64(422) {
		t.Errorf("code = %v, want 422", result["code"])
	}
	if result["message"] != "Validation failed" {
		t.Errorf("message = %v, want 'Validation failed'", result["message"])
	}
	errs, ok := result["errors"].([]any)
	if !ok {
		t.Fatal("errors should be an array")
	}
	if len(errs) != 2 {
		t.Fatalf("errors length = %d, want 2", len(errs))
	}
	first := errs[0].(map[string]any)
	if first["field"] != "email" {
		t.Errorf("first error field = %v, want email", first["field"])
	}
	if first["rule"] != "required" {
		t.Errorf("first error rule = %v, want required", first["rule"])
	}
}

func TestValidationError_MarshalJSON_Empty(t *testing.T) {
	ve := &ValidationError{Errors: []FieldError{}}
	data, err := ve.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	errs := result["errors"].([]any)
	if len(errs) != 0 {
		t.Errorf("errors length = %d, want 0", len(errs))
	}
}

// ---------------------------------------------------------------------------
// Task 13.2: Built-in rule tests (18 rules)
// ---------------------------------------------------------------------------

func TestValidateRequired(t *testing.T) {
	tests := []struct {
		value any
		want  bool
	}{
		{"hello", true},
		{"", false},
		{0, false},
		{1, true},
		{nil, false},
		{[]int{1}, true},
		{[]int{}, false},
		{true, true},
		{false, false},
	}
	for _, tt := range tests {
		if got := validateRequired(tt.value, ""); got != tt.want {
			t.Errorf("validateRequired(%v) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestValidateMin(t *testing.T) {
	tests := []struct {
		value any
		param string
		want  bool
	}{
		{"hello", "3", true}, // len 5 >= 3
		{"hi", "3", false},   // len 2 < 3
		{10, "5", true},      // 10 >= 5
		{3, "5", false},      // 3 < 5
		{uint(10), "5", true},
		{3.14, "3.0", true},
		{2.5, "3.0", false},
	}
	for _, tt := range tests {
		if got := validateMin(tt.value, tt.param); got != tt.want {
			t.Errorf("validateMin(%v, %q) = %v, want %v", tt.value, tt.param, got, tt.want)
		}
	}
}

func TestValidateMax(t *testing.T) {
	tests := []struct {
		value any
		param string
		want  bool
	}{
		{"hi", "3", true},     // len 2 <= 3
		{"hello", "3", false}, // len 5 > 3
		{3, "5", true},        // 3 <= 5
		{10, "5", false},      // 10 > 5
		{uint(3), "5", true},
		{2.5, "3.0", true},
		{3.5, "3.0", false},
	}
	for _, tt := range tests {
		if got := validateMax(tt.value, tt.param); got != tt.want {
			t.Errorf("validateMax(%v, %q) = %v, want %v", tt.value, tt.param, got, tt.want)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		value any
		want  bool
	}{
		{"user@example.com", true},
		{"bad", false},
		{"", false},
		{123, false}, // non-string
	}
	for _, tt := range tests {
		if got := validateEmail(tt.value, ""); got != tt.want {
			t.Errorf("validateEmail(%v) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		value any
		want  bool
	}{
		{"https://example.com", true},
		{"http://localhost:8080/path", true},
		{"not-a-url", false},
		{"", false},
		{123, false},
	}
	for _, tt := range tests {
		if got := validateURL(tt.value, ""); got != tt.want {
			t.Errorf("validateURL(%v) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestValidateOneOf(t *testing.T) {
	tests := []struct {
		value any
		param string
		want  bool
	}{
		{"admin", "admin user guest", true},
		{"other", "admin user guest", false},
		{1, "1 2 3", true},
		{4, "1 2 3", false},
	}
	for _, tt := range tests {
		if got := validateOneOf(tt.value, tt.param); got != tt.want {
			t.Errorf("validateOneOf(%v, %q) = %v, want %v", tt.value, tt.param, got, tt.want)
		}
	}
}

func TestValidateLen(t *testing.T) {
	tests := []struct {
		value any
		param string
		want  bool
	}{
		{"abc", "3", true},
		{"ab", "3", false},
		{[]int{1, 2}, "2", true},
		{[]int{1}, "2", false},
	}
	for _, tt := range tests {
		if got := validateLen(tt.value, tt.param); got != tt.want {
			t.Errorf("validateLen(%v, %q) = %v, want %v", tt.value, tt.param, got, tt.want)
		}
	}
}

func TestValidateGT(t *testing.T) {
	if !validateGT(10, "5") {
		t.Error("10 > 5 should be true")
	}
	if validateGT(5, "5") {
		t.Error("5 > 5 should be false")
	}
	if !validateGT(uint(10), "5") {
		t.Error("uint(10) > 5 should be true")
	}
	if !validateGT(3.5, "3.0") {
		t.Error("3.5 > 3.0 should be true")
	}
	// string length
	if !validateGT("hello", "3") {
		t.Error("len(hello)=5 > 3 should be true")
	}
}

func TestValidateGTE(t *testing.T) {
	if !validateGTE(5, "5") {
		t.Error("5 >= 5 should be true")
	}
	if validateGTE(4, "5") {
		t.Error("4 >= 5 should be false")
	}
}

func TestValidateLT(t *testing.T) {
	if !validateLT(3, "5") {
		t.Error("3 < 5 should be true")
	}
	if validateLT(5, "5") {
		t.Error("5 < 5 should be false")
	}
}

func TestValidateLTE(t *testing.T) {
	if !validateLTE(5, "5") {
		t.Error("5 <= 5 should be true")
	}
	if validateLTE(6, "5") {
		t.Error("6 <= 5 should be false")
	}
}

func TestValidateUUID(t *testing.T) {
	if !validateUUID("550e8400-e29b-41d4-a716-446655440000", "") {
		t.Error("valid UUID should pass")
	}
	if validateUUID("not-a-uuid", "") {
		t.Error("invalid UUID should fail")
	}
	if validateUUID("550e8400-e29b-41d4-a716-44665544000", "") {
		t.Error("short UUID should fail")
	}
	if validateUUID(123, "") {
		t.Error("non-string should fail")
	}
}

func TestValidateAlpha(t *testing.T) {
	if !validateAlpha("Hello", "") {
		t.Error("Hello should be alpha")
	}
	if validateAlpha("Hello123", "") {
		t.Error("Hello123 should not be alpha")
	}
	if validateAlpha("", "") {
		t.Error("empty should fail")
	}
}

func TestValidateAlphanum(t *testing.T) {
	if !validateAlphanum("Hello123", "") {
		t.Error("Hello123 should be alphanum")
	}
	if validateAlphanum("Hello 123", "") {
		t.Error("space should fail")
	}
	if validateAlphanum("", "") {
		t.Error("empty should fail")
	}
}

func TestValidateNumeric(t *testing.T) {
	if !validateNumeric("12345", "") {
		t.Error("12345 should be numeric")
	}
	if !validateNumeric("-123", "") {
		t.Error("-123 should be numeric")
	}
	if validateNumeric("12.3", "") {
		t.Error("12.3 should fail (has dot)")
	}
	if validateNumeric("", "") {
		t.Error("empty should fail")
	}
	if validateNumeric("-", "") {
		t.Error("just minus should fail")
	}
}

func TestValidateContains(t *testing.T) {
	if !validateContains("hello world", "world") {
		t.Error("should contain 'world'")
	}
	if validateContains("hello", "world") {
		t.Error("should not contain 'world'")
	}
}

func TestValidateStartsWith(t *testing.T) {
	if !validateStartsWith("hello world", "hello") {
		t.Error("should start with 'hello'")
	}
	if validateStartsWith("world hello", "hello") {
		t.Error("should not start with 'hello'")
	}
}

func TestValidateEndsWith(t *testing.T) {
	if !validateEndsWith("hello world", "world") {
		t.Error("should end with 'world'")
	}
	if validateEndsWith("world hello", "world") {
		t.Error("should not end with 'world'")
	}
}

// ---------------------------------------------------------------------------
// Task 13.3: Custom rule registration via Register()
// ---------------------------------------------------------------------------

func TestValidator_Register_CustomRule(t *testing.T) {
	v := NewValidator()
	v.Register("even", func(value any, param string) bool {
		n, ok := value.(int)
		if !ok {
			return false
		}
		return n%2 == 0
	})

	if _, ok := v.rules["even"]; !ok {
		t.Fatal("custom rule 'even' should be registered")
	}
	if !v.rules["even"](4, "") {
		t.Error("4 should be even")
	}
	if v.rules["even"](3, "") {
		t.Error("3 should not be even")
	}
}

// ---------------------------------------------------------------------------
// Task 13.4: Message override via Messages() and message:"text" struct tag
// ---------------------------------------------------------------------------

func TestValidator_Messages_Override(t *testing.T) {
	v := NewValidator()
	v.Messages(map[string]string{
		"required": "{field} ห้ามว่าง",
	})
	if v.messages["required"] != "{field} ห้ามว่าง" {
		t.Errorf("messages[required] = %q, want Thai override", v.messages["required"])
	}
	// Other messages should remain default
	if v.messages["email"] != "{field} must be a valid email address" {
		t.Error("email message should remain default")
	}
}

func TestValidator_MessageTag_Override(t *testing.T) {
	type Input struct {
		Name string `json:"name" validate:"required" message:"กรุณากรอกชื่อ"`
	}
	v := NewValidator()
	validators := buildValidators[Input](v)
	if len(validators) != 1 {
		t.Fatalf("expected 1 validator, got %d", len(validators))
	}
	if validators[0].customMsg != "กรุณากรอกชื่อ" {
		t.Errorf("customMsg = %q, want Thai message", validators[0].customMsg)
	}

	// Validate with empty name — should use custom message
	val := reflect.ValueOf(Input{Name: ""})
	ve := validate(validators, val, v.messages)
	if ve == nil {
		t.Fatal("expected validation error")
	}
	if ve.Errors[0].Message != "กรุณากรอกชื่อ" {
		t.Errorf("message = %q, want custom Thai message", ve.Errors[0].Message)
	}
}

// ---------------------------------------------------------------------------
// Task 13.5: buildValidators tests
// ---------------------------------------------------------------------------

func TestBuildValidators_MultiField(t *testing.T) {
	type Input struct {
		Name  string `json:"name" validate:"required,min=2"`
		Email string `json:"email" validate:"required,email"`
	}
	v := NewValidator()
	validators := buildValidators[Input](v)
	if len(validators) != 2 {
		t.Fatalf("expected 2 validators, got %d", len(validators))
	}
	if validators[0].fieldName != "name" {
		t.Errorf("first field = %q, want name", validators[0].fieldName)
	}
	if len(validators[0].rules) != 2 {
		t.Errorf("name rules = %d, want 2", len(validators[0].rules))
	}
	if validators[1].fieldName != "email" {
		t.Errorf("second field = %q, want email", validators[1].fieldName)
	}
}

func TestBuildValidators_UnknownRulePanics(t *testing.T) {
	type Input struct {
		Name string `validate:"nonexistent"`
	}
	v := NewValidator()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unknown rule")
		}
	}()
	buildValidators[Input](v)
}

func TestBuildValidators_NilValidator(t *testing.T) {
	type Input struct {
		Name string `validate:"required"`
	}
	validators := buildValidators[Input](nil)
	if validators != nil {
		t.Error("nil validator should return nil")
	}
}

func TestBuildValidators_FieldNameFallback(t *testing.T) {
	type Input struct {
		UserName string `validate:"required"` // no json tag → lowercased "username"
	}
	v := NewValidator()
	validators := buildValidators[Input](v)
	if validators[0].fieldName != "username" {
		t.Errorf("fieldName = %q, want 'username'", validators[0].fieldName)
	}
}

func TestBuildValidators_JsonTagStripping(t *testing.T) {
	type Input struct {
		Name string `json:"name,omitempty" validate:"required"`
	}
	v := NewValidator()
	validators := buildValidators[Input](v)
	if validators[0].fieldName != "name" {
		t.Errorf("fieldName = %q, want 'name' (stripped omitempty)", validators[0].fieldName)
	}
}

// ---------------------------------------------------------------------------
// Task 13.6: validate() tests
// ---------------------------------------------------------------------------

func TestValidate_AllPass(t *testing.T) {
	type Input struct {
		Name  string `json:"name" validate:"required,min=2"`
		Email string `json:"email" validate:"required,email"`
	}
	v := NewValidator()
	validators := buildValidators[Input](v)
	val := reflect.ValueOf(Input{Name: "John", Email: "john@example.com"})
	ve := validate(validators, val, v.messages)
	if ve != nil {
		t.Errorf("expected nil, got %v", ve)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	type Input struct {
		Name  string `json:"name" validate:"required,min=2"`
		Email string `json:"email" validate:"required,email"`
	}
	v := NewValidator()
	validators := buildValidators[Input](v)
	val := reflect.ValueOf(Input{Name: "", Email: ""})
	ve := validate(validators, val, v.messages)
	if ve == nil {
		t.Fatal("expected validation errors")
	}
	// Name: required fails, min fails. Email: required fails, email fails.
	if len(ve.Errors) != 4 {
		t.Errorf("error count = %d, want 4", len(ve.Errors))
	}
}

func TestValidate_CustomMessageUsed(t *testing.T) {
	type Input struct {
		Name string `json:"name" validate:"required" message:"name cannot be empty"`
	}
	v := NewValidator()
	validators := buildValidators[Input](v)
	val := reflect.ValueOf(Input{Name: ""})
	ve := validate(validators, val, v.messages)
	if ve == nil {
		t.Fatal("expected validation error")
	}
	if ve.Errors[0].Message != "name cannot be empty" {
		t.Errorf("message = %q, want custom message", ve.Errors[0].Message)
	}
}

func TestValidate_EmptyValidators(t *testing.T) {
	ve := validate(nil, reflect.Value{}, nil)
	if ve != nil {
		t.Error("empty validators should return nil")
	}
}

func TestFormatMessage_UnknownRule(t *testing.T) {
	msg := formatMessage(defaultMessages(), "unknown_rule", "field", "")
	if msg != "field is invalid" {
		t.Errorf("unknown rule message = %q, want 'field is invalid'", msg)
	}
}

// ---------------------------------------------------------------------------
// Task 5.3: File upload validation rules (max_size, mime, required for *FileUpload)
// ---------------------------------------------------------------------------

func TestValidateMaxSize(t *testing.T) {
	tests := []struct {
		name  string
		value any
		param string
		want  bool
	}{
		{"under limit", &FileUpload{Size: 1024}, "5mb", true},
		{"at limit", &FileUpload{Size: 5 * 1024 * 1024}, "5mb", true},
		{"over limit", &FileUpload{Size: 6 * 1024 * 1024}, "5mb", false},
		{"KB suffix", &FileUpload{Size: 500}, "1kb", true},
		{"KB over", &FileUpload{Size: 2000}, "1kb", false},
		{"GB suffix", &FileUpload{Size: 1024 * 1024 * 1024}, "1gb", true},
		{"GB over", &FileUpload{Size: 2 * 1024 * 1024 * 1024}, "1gb", false},
		{"nil FileUpload passes", (*FileUpload)(nil), "5mb", true},
		{"non-FileUpload passes", "not a file", "5mb", true},
		{"invalid param", &FileUpload{Size: 100}, "invalid", false},
		{"zero size", &FileUpload{Size: 0}, "1kb", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateMaxSize(tt.value, tt.param); got != tt.want {
				t.Errorf("validateMaxSize(%v, %q) = %v, want %v", tt.value, tt.param, got, tt.want)
			}
		})
	}
}

func TestValidateMime(t *testing.T) {
	tests := []struct {
		name  string
		value any
		param string
		want  bool
	}{
		{"exact match pdf", &FileUpload{ContentType: "application/pdf"}, "application/pdf", true},
		{"exact mismatch", &FileUpload{ContentType: "application/pdf"}, "image/png", false},
		{"wildcard image/* matches png", &FileUpload{ContentType: "image/png"}, "image/*", true},
		{"wildcard image/* matches jpeg", &FileUpload{ContentType: "image/jpeg"}, "image/*", true},
		{"wildcard image/* no match text", &FileUpload{ContentType: "text/plain"}, "image/*", false},
		{"wildcard application/*", &FileUpload{ContentType: "application/json"}, "application/*", true},
		{"nil FileUpload passes", (*FileUpload)(nil), "image/*", true},
		{"non-FileUpload passes", "not a file", "image/*", true},
		{"empty content type", &FileUpload{ContentType: ""}, "image/*", false},
		{"exact match empty param", &FileUpload{ContentType: "image/png"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateMime(tt.value, tt.param); got != tt.want {
				t.Errorf("validateMime(%v, %q) = %v, want %v", tt.value, tt.param, got, tt.want)
			}
		})
	}
}

func TestValidateRequired_FileUpload(t *testing.T) {
	// nil *FileUpload should fail required
	var nilFile *FileUpload
	if validateRequired(nilFile, "") {
		t.Error("nil *FileUpload should fail required")
	}

	// non-nil *FileUpload should pass required
	file := &FileUpload{Name: "test.png", Size: 100, ContentType: "image/png"}
	if !validateRequired(file, "") {
		t.Error("non-nil *FileUpload should pass required")
	}
}
