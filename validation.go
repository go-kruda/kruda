package kruda

import (
	"fmt"
	"net/mail"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	krudajson "github.com/go-kruda/kruda/json"
)

// ValidatorFunc is the signature for validation rule functions.
// value is the field value to validate, param is the rule parameter
// (e.g. "18" for min=18, "" for required).
// Returns true if valid, false if invalid.
type ValidatorFunc func(value any, param string) bool

// FieldError represents a single field validation failure.
type FieldError struct {
	Field   string `json:"field"`   // json tag name or struct field name
	Rule    string `json:"rule"`    // "required", "min", "email", etc.
	Param   string `json:"param"`   // "18" for min=18, "" for required
	Message string `json:"message"` // "email is required"
	Value   string `json:"value"`   // stringified rejected value
}

// ValidationError holds structured validation errors.
// Implements error and json.Marshaler.
type ValidationError struct {
	Errors []FieldError `json:"errors"`
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Message
	}
	return fmt.Sprintf("validation failed: %d errors", len(e.Errors))
}

// MarshalJSON produces the structured JSON response.
// Uses the build-tag-selected JSON engine (sonic when available, else encoding/json).
func (e *ValidationError) MarshalJSON() ([]byte, error) {
	type response struct {
		Code    int          `json:"code"`
		Message string       `json:"message"`
		Errors  []FieldError `json:"errors"`
	}
	return krudajson.Marshal(response{
		Code:    422,
		Message: "Validation failed",
		Errors:  e.Errors,
	})
}

// Validator holds registered rules and message templates.
// Created once per App, configured at startup.
type Validator struct {
	rules    map[string]ValidatorFunc
	messages map[string]string
}

// NewValidator creates a Validator with built-in rules and default messages.
func NewValidator() *Validator {
	return &Validator{
		rules:    builtinRules(),
		messages: defaultMessages(),
	}
}

// Register adds a custom validation rule. Chainable.
func (v *Validator) Register(name string, fn ValidatorFunc) *Validator {
	v.rules[name] = fn
	return v
}

// Messages overrides default message templates. Chainable.
func (v *Validator) Messages(overrides map[string]string) *Validator {
	for k, msg := range overrides {
		v.messages[k] = msg
	}
	return v
}

// --- Pre-compiled validator chain ---

// fieldValidator is pre-compiled at registration time.
type fieldValidator struct {
	index     int         // struct field index
	fieldName string      // json tag name or lowercased struct field name
	rules     []ruleEntry // pre-parsed rules in order
	customMsg string      // from `message:"..."` tag, empty if not set
}

// ruleEntry is a single parsed validation rule.
type ruleEntry struct {
	name  string        // "required", "min", "email"
	param string        // "18" for min=18, "" for required
	fn    ValidatorFunc // pre-looked-up function
}

// buildValidators reflects on type T and pre-compiles validator chains.
// Called at route registration time. Returns nil if no validate tags found.
func buildValidators[T any](v *Validator) []fieldValidator {
	if v == nil {
		return nil
	}

	t := reflect.TypeOf((*T)(nil)).Elem()
	var validators []fieldValidator

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("validate")
		if tag == "" {
			continue
		}

		// Determine field name for error messages
		name := field.Tag.Get("json")
		if name == "" || name == "-" {
			name = strings.ToLower(field.Name)
		}
		// Strip json tag options (e.g. "name,omitempty" → "name")
		if idx := strings.IndexByte(name, ','); idx != -1 {
			name = name[:idx]
		}

		fv := fieldValidator{
			index:     i,
			fieldName: name,
			customMsg: field.Tag.Get("message"),
		}

		// Parse rules: "required,min=2,email" → [{required,""}, {min,"2"}, {email,""}]
		for _, rule := range strings.Split(tag, ",") {
			rule = strings.TrimSpace(rule)
			ruleName, ruleParam, _ := strings.Cut(rule, "=")
			fn, ok := v.rules[ruleName]
			if !ok {
				panic(fmt.Sprintf("kruda: unknown validation rule %q on field %s", ruleName, field.Name))
			}
			fv.rules = append(fv.rules, ruleEntry{name: ruleName, param: ruleParam, fn: fn})
		}

		validators = append(validators, fv)
	}

	return validators
}

// validate runs pre-compiled validators against a struct value.
// Returns nil if all valid, or *ValidationError with all failures.
func validate(validators []fieldValidator, v reflect.Value, messages map[string]string) *ValidationError {
	if len(validators) == 0 {
		return nil
	}

	var errs []FieldError

	for _, fv := range validators {
		fieldVal := v.Field(fv.index)
		value := fieldVal.Interface()

		for _, rule := range fv.rules {
			if rule.fn(value, rule.param) {
				continue // valid
			}

			msg := fv.customMsg
			if msg == "" {
				msg = formatMessage(messages, rule.name, fv.fieldName, rule.param)
			}

			errs = append(errs, FieldError{
				Field:   fv.fieldName,
				Rule:    rule.name,
				Param:   rule.param,
				Message: msg,
				Value:   fmt.Sprintf("%v", value),
			})
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{Errors: errs}
}

// formatMessage generates a human-readable message from templates.
func formatMessage(messages map[string]string, rule, field, param string) string {
	tmpl, ok := messages[rule]
	if !ok {
		return field + " is invalid"
	}
	msg := strings.ReplaceAll(tmpl, "{field}", field)
	msg = strings.ReplaceAll(msg, "{param}", param)
	return msg
}

// --- Built-in rules ---

func builtinRules() map[string]ValidatorFunc {
	return map[string]ValidatorFunc{
		"required":   validateRequired,
		"min":        validateMin,
		"max":        validateMax,
		"email":      validateEmail,
		"url":        validateURL,
		"oneof":      validateOneOf,
		"len":        validateLen,
		"gt":         validateGT,
		"gte":        validateGTE,
		"lt":         validateLT,
		"lte":        validateLTE,
		"uuid":       validateUUID,
		"alpha":      validateAlpha,
		"alphanum":   validateAlphanum,
		"numeric":    validateNumeric,
		"contains":   validateContains,
		"startswith": validateStartsWith,
		"endswith":   validateEndsWith,
		"max_size":   validateMaxSize,
		"mime":       validateMime,
	}
}

func defaultMessages() map[string]string {
	return map[string]string{
		"required":   "{field} is required",
		"min":        "{field} must be at least {param}",
		"max":        "{field} must be at most {param}",
		"email":      "{field} must be a valid email address",
		"url":        "{field} must be a valid URL",
		"oneof":      "{field} must be one of: {param}",
		"len":        "{field} must be exactly {param} characters",
		"gt":         "{field} must be greater than {param}",
		"gte":        "{field} must be greater than or equal to {param}",
		"lt":         "{field} must be less than {param}",
		"lte":        "{field} must be less than or equal to {param}",
		"uuid":       "{field} must be a valid UUID",
		"alpha":      "{field} must contain only letters",
		"alphanum":   "{field} must contain only letters and digits",
		"numeric":    "{field} must be numeric",
		"contains":   "{field} must contain {param}",
		"startswith": "{field} must start with {param}",
		"endswith":   "{field} must end with {param}",
		"max_size":   "{field} must be at most {param}",
		"mime":       "{field} must be of type {param}",
	}
}

// --- Rule implementations ---

// validateRequired checks that a value is not zero.
func validateRequired(value any, _ string) bool {
	if value == nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return v.Len() > 0
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() > 0
	case reflect.Ptr, reflect.Interface:
		return !v.IsNil()
	default:
		return !v.IsZero()
	}
}

// validateMin checks minimum value (numeric) or length (string/slice).
func validateMin(value any, param string) bool {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return float64(v.Len()) >= n
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Use integer comparison to avoid float64 precision loss for large values
		if ni, err := strconv.ParseInt(param, 10, 64); err == nil {
			return v.Int() >= ni
		}
		return float64(v.Int()) >= n
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if nu, err := strconv.ParseUint(param, 10, 64); err == nil {
			return v.Uint() >= nu
		}
		return float64(v.Uint()) >= n
	case reflect.Float32, reflect.Float64:
		return v.Float() >= n
	case reflect.Slice, reflect.Map, reflect.Array:
		return float64(v.Len()) >= n
	}
	return false
}

// validateMax checks maximum value (numeric) or length (string/slice).
func validateMax(value any, param string) bool {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return float64(v.Len()) <= n
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ni, err := strconv.ParseInt(param, 10, 64); err == nil {
			return v.Int() <= ni
		}
		return float64(v.Int()) <= n
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if nu, err := strconv.ParseUint(param, 10, 64); err == nil {
			return v.Uint() <= nu
		}
		return float64(v.Uint()) <= n
	case reflect.Float32, reflect.Float64:
		return v.Float() <= n
	case reflect.Slice, reflect.Map, reflect.Array:
		return float64(v.Len()) <= n
	}
	return false
}

// validateEmail checks for a valid email format using net/mail.
func validateEmail(value any, _ string) bool {
	s, ok := value.(string)
	if !ok || s == "" {
		return false
	}
	_, err := mail.ParseAddress(s)
	return err == nil
}

// validateURL checks for a valid URL format using net/url.
func validateURL(value any, _ string) bool {
	s, ok := value.(string)
	if !ok || s == "" {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// validateOneOf checks if value is one of the space-separated options.
func validateOneOf(value any, param string) bool {
	s := fmt.Sprintf("%v", value)
	for _, opt := range strings.Fields(param) {
		if s == opt {
			return true
		}
	}
	return false
}

// validateLen checks exact length (string) or exact value count (slice/map).
func validateLen(value any, param string) bool {
	n, err := strconv.Atoi(param)
	if err != nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == n
	}
	return false
}

// validateGT checks greater than (numeric).
func validateGT(value any, param string) bool {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ni, err := strconv.ParseInt(param, 10, 64); err == nil {
			return v.Int() > ni
		}
		return float64(v.Int()) > n
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if nu, err := strconv.ParseUint(param, 10, 64); err == nil {
			return v.Uint() > nu
		}
		return float64(v.Uint()) > n
	case reflect.Float32, reflect.Float64:
		return v.Float() > n
	case reflect.String:
		return float64(v.Len()) > n
	}
	return false
}

// validateGTE checks greater than or equal (numeric).
func validateGTE(value any, param string) bool {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ni, err := strconv.ParseInt(param, 10, 64); err == nil {
			return v.Int() >= ni
		}
		return float64(v.Int()) >= n
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if nu, err := strconv.ParseUint(param, 10, 64); err == nil {
			return v.Uint() >= nu
		}
		return float64(v.Uint()) >= n
	case reflect.Float32, reflect.Float64:
		return v.Float() >= n
	case reflect.String:
		return float64(v.Len()) >= n
	}
	return false
}

// validateLT checks less than (numeric).
func validateLT(value any, param string) bool {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ni, err := strconv.ParseInt(param, 10, 64); err == nil {
			return v.Int() < ni
		}
		return float64(v.Int()) < n
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if nu, err := strconv.ParseUint(param, 10, 64); err == nil {
			return v.Uint() < nu
		}
		return float64(v.Uint()) < n
	case reflect.Float32, reflect.Float64:
		return v.Float() < n
	case reflect.String:
		return float64(v.Len()) < n
	}
	return false
}

// validateLTE checks less than or equal (numeric).
func validateLTE(value any, param string) bool {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ni, err := strconv.ParseInt(param, 10, 64); err == nil {
			return v.Int() <= ni
		}
		return float64(v.Int()) <= n
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if nu, err := strconv.ParseUint(param, 10, 64); err == nil {
			return v.Uint() <= nu
		}
		return float64(v.Uint()) <= n
	case reflect.Float32, reflect.Float64:
		return v.Float() <= n
	case reflect.String:
		return float64(v.Len()) <= n
	}
	return false
}

// validateUUID checks for UUID format (8-4-4-4-12 hex).
func validateUUID(value any, _ string) bool {
	s, ok := value.(string)
	if !ok || len(s) != 36 {
		return false
	}
	for i := 0; i < 36; i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// validateAlpha checks that string contains only letters.
func validateAlpha(value any, _ string) bool {
	s, ok := value.(string)
	if !ok || s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

// validateAlphanum checks that string contains only letters and digits.
func validateAlphanum(value any, _ string) bool {
	s, ok := value.(string)
	if !ok || s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// validateNumeric checks that string contains only digits (and optional leading minus).
func validateNumeric(value any, _ string) bool {
	s, ok := value.(string)
	if !ok || s == "" {
		return false
	}
	start := 0
	if s[0] == '-' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// validateContains checks that string contains the given substring.
func validateContains(value any, param string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return strings.Contains(s, param)
}

// validateStartsWith checks that string starts with the given prefix.
func validateStartsWith(value any, param string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return strings.HasPrefix(s, param)
}

// validateEndsWith checks that string ends with the given suffix.
func validateEndsWith(value any, param string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return strings.HasSuffix(s, param)
}

// validateMaxSize checks file size against a human-readable limit.
// Works with *FileUpload fields. Param format: "5mb", "500kb", "1gb".
func validateMaxSize(value any, param string) bool {
	fu, ok := value.(*FileUpload)
	if !ok || fu == nil {
		return true // nil handled by required rule
	}
	maxBytes, err := parseSize(param) // reuse from config.go
	if err != nil {
		return false
	}
	return fu.Size <= int64(maxBytes)
}

// validateMime checks file Content-Type against a pattern.
// Supports wildcard: "image/*" matches "image/png", "image/jpeg".
func validateMime(value any, param string) bool {
	fu, ok := value.(*FileUpload)
	if !ok || fu == nil {
		return true // nil handled by required rule
	}
	if strings.Contains(param, "/*") {
		prefix := strings.TrimSuffix(param, "/*")
		return strings.HasPrefix(fu.ContentType, prefix+"/")
	}
	return fu.ContentType == param
}
