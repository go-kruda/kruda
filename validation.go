package kruda

import (
	"fmt"
	"log/slog"
	"net/mail"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

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
	rules       map[string]ValidatorFunc
	messages    map[string]string
	customRules map[string]bool
}

// NewValidator creates a Validator with built-in rules and default messages.
func NewValidator() *Validator {
	return &Validator{
		rules:       builtinRules(),
		messages:    defaultMessages(),
		customRules: make(map[string]bool),
	}
}

// Register adds a custom validation rule. Chainable.
func (v *Validator) Register(name string, fn ValidatorFunc) *Validator {
	v.rules[name] = fn
	v.customRules[name] = true
	return v
}

// Messages overrides default message templates. Chainable.
func (v *Validator) Messages(overrides map[string]string) *Validator {
	for k, msg := range overrides {
		v.messages[k] = msg
	}
	return v
}

// fieldValidator is pre-compiled at registration time.
type fieldValidator struct {
	index     int         // struct field index
	fieldName string      // json tag name or lowercased struct field name
	rules     []ruleEntry // pre-parsed rules in order, applied to the field itself
	elemRules []ruleEntry // rules after `dive`, applied to each element
	customMsg string      // from `message:"..."` tag, empty if not set
	omitEmpty bool        // `omitempty`: a zero value skips the field's rules
	checks    []func(reflect.Value) bool
	// elemChecks mirrors elemRules the way checks mirrors rules: when every
	// element passes them, the element is never boxed.
	elemChecks []func(reflect.Value) bool
}

// ruleEntry is a single parsed validation rule.
type ruleEntry struct {
	name  string        // "required", "min", "email"
	param string        // "18" for min=18, "" for required
	fn    ValidatorFunc // pre-looked-up function
}

// validationModifiers are the tag words that carry no ValidatorFunc and instead
// change how the rules around them apply. They are handled structurally by
// buildValidators, so they never appear in the rules map — this is the one place
// that names them, so the parser, the unknown-rule warning and the documentation
// guard cannot drift apart.
var validationModifiers = [...]string{"omitempty", "dive"}

// warnedUnknownRules keeps the warning to one line per rule per field, so a type
// registered on many routes does not repeat it.
var warnedUnknownRules sync.Map

// warnUnknownRule reports a validate tag naming a rule this package does not
// implement. That rule is skipped; the field's other rules still apply.
func warnUnknownRule(v *Validator, rule, typeName, fieldName string) {
	key := rule + "\x00" + typeName + "\x00" + fieldName
	if _, loaded := warnedUnknownRules.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	// The rules this Validator has, not the built-in set: an application that
	// registered its own would otherwise be told they are unsupported, which is
	// the opposite of true and unhelpful precisely when someone has misspelled
	// one of their own rules.
	slog.Warn("kruda: unknown validation rule in a validate tag — that rule is ignored, the field's other rules still apply",
		"rule", rule, "type", typeName, "field", fieldName,
		"available", strings.Join(v.RuleNames(), ","),
		"modifiers", strings.Join(validationModifiers[:], ","))
}

// RuleNames lists the validation rules this Validator recognises, sorted —
// the built-in set plus anything added with Register.
//
// The tag syntax resembles go-playground/validator, but that library has far more
// rules, so a tag carried over from it may name one that does not exist here. A
// rule this Validator does not have is skipped with a startup warning rather than
// failing the build, so RuleNames is the way to check what a tag can actually
// use.
//
// The modifiers `omitempty` and `dive` are supported and deliberately absent from
// this list: they are not rules, carry no ValidatorFunc, and cannot be added with
// Register. `omitempty` skips a field's rules when its value is the zero value;
// `dive` applies every rule after it to each element of a slice, array or map
// instead of to the container.
func (v *Validator) RuleNames() []string {
	names := make([]string, 0, len(v.rules))
	for n := range v.rules {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
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
		//
		// target is where the next rule lands. `dive` moves it from the field's
		// own rules to the per-element ones, so the split is resolved here rather
		// than re-derived on every request.
		target := &fv.rules
		for _, rule := range strings.Split(tag, ",") {
			rule = strings.TrimSpace(rule)
			ruleName, ruleParam, _ := strings.Cut(rule, "=")

			// omitempty and dive are modifiers, not rules: they carry no
			// ValidatorFunc and change how the rules around them apply. That
			// distinction is why they cannot be left to the unknown-rule path —
			// skipping a constraint only relaxes validation, but skipping a
			// modifier applies the rules after it in the wrong place. Dropping
			// omitempty makes an optional field behave as required; dropping
			// dive runs an element rule against the container, which no slice
			// can satisfy.
			// The cases here are validationModifiers; a test pins the two lists
			// together so a modifier cannot be dropped while docs still teach it.
			switch ruleName {
			case "omitempty":
				fv.omitEmpty = true
				continue
			case "dive":
				target = &fv.elemRules
				if k := diveKind(field.Type); k == reflect.Invalid {
					warnDiveOnNonCollection(t.Name(), field.Name, field.Type.String())
				}
				continue
			}

			fn, ok := v.rules[ruleName]
			if !ok {
				// Warn and skip rather than panic. This package implements 20
				// rules and borrows its tag syntax from go-playground/validator,
				// which has hundreds — omitempty alone is close to universal in Go
				// codebases. While validation was opt-in those tags sat inert, so
				// applications carrying them run today. Panicking here would turn
				// validation-by-default into a boot failure for them, which is
				// worse than one rule going unenforced and being said so.
				//
				// Nothing that boots today changes: an application with an unknown
				// rule and a Validator already configured panics as it is.
				warnUnknownRule(v, ruleName, t.Name(), field.Name)
				continue
			}
			*target = append(*target, ruleEntry{name: ruleName, param: ruleParam, fn: fn})
		}

		// A field whose every rule was skipped validates nothing, so it must not
		// be recorded as a validator. len(validators) is what sets hasValidate on
		// the route, which is what puts a 422 in the generated OpenAPI document —
		// keeping an empty entry would advertise a response the handler can never
		// produce. A lone `omitempty` lands here too: it constrains nothing.
		if len(fv.rules) == 0 && len(fv.elemRules) == 0 {
			continue
		}
		if field.IsExported() {
			fv.checks = compileRules(v, field.Type, fv.rules)
			if et := diveElem(field.Type); et != nil {
				fv.elemChecks = compileRules(v, et, fv.elemRules)
			}
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

		// Tested before the value is boxed: an empty optional field costs a zero
		// check and skips the Interface() allocation entirely.
		if fv.omitEmpty && fieldVal.IsZero() {
			continue
		}

		// Only pure built-in chains use the unboxed success path. Failures and
		// custom rules retain the original value snapshot and error ordering.
		if len(fv.rules) > 0 && !passesAll(fv.checks, fieldVal) {
			value := fieldVal.Interface()
			for _, rule := range fv.rules {
				if rule.fn(value, rule.param) {
					continue
				}
				errs = append(errs, fv.fieldError(messages, rule, fv.fieldName, value))
			}
		}

		if len(fv.elemRules) > 0 {
			errs = fv.validateElems(messages, fieldVal, errs)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{Errors: errs}
}

// fieldError builds the failure record for one rule. name is passed separately
// so a `dive` failure can report the element it came from rather than the
// container.
func (fv *fieldValidator) fieldError(messages map[string]string, rule ruleEntry, name string, value any) FieldError {
	msg := fv.customMsg
	if msg == "" {
		msg = formatMessage(messages, rule.name, name, rule.param)
	}
	return FieldError{
		Field:   name,
		Rule:    rule.name,
		Param:   rule.param,
		Message: msg,
		Value:   fmt.Sprintf("%v", value),
	}
}

// validateElems applies the rules after `dive` to each element of a slice,
// array or map. Errors name the element — `tags[2]`, `limits[free]` — because a
// message that only says "tags" leaves the caller to find which entry failed.
//
// A field that is not a collection produced a startup warning at build time and
// is skipped here: running an element rule against the container is what the
// dive support exists to prevent.
func (fv *fieldValidator) validateElems(messages map[string]string, fieldVal reflect.Value, errs []FieldError) []FieldError {
	if fieldVal.Kind() == reflect.Pointer {
		if fieldVal.IsNil() {
			return errs
		}
		fieldVal = fieldVal.Elem()
	}

	switch fieldVal.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < fieldVal.Len(); i++ {
			errs = fv.applyElemRules(messages, fieldVal.Index(i), i, reflect.Value{}, errs)
		}
	case reflect.Map:
		// A key type with its own formatting runs user code, so its label is
		// taken before the element rules run, as it always was.
		eager := formatsItself(fieldVal.Type().Key())
		iter := fieldVal.MapRange()
		for iter.Next() {
			key := iter.Key()
			if eager {
				// The label is taken before the value, as it always was: a
				// key's String method may change what the value holds.
				name := fv.elemName(0, key)
				errs = fv.applyElemRulesNamed(messages, iter.Value(), name, errs)
				continue
			}
			errs = fv.applyElemRules(messages, iter.Value(), 0, key, errs)
		}
	}
	return errs
}

// applyElemRules checks one element. Its name (`tags[2]`, `limits[free]`) is
// built on the first failure, so a valid collection costs no labels.
func (fv *fieldValidator) applyElemRules(messages map[string]string, elem reflect.Value, idx int, key reflect.Value, errs []FieldError) []FieldError {
	if passesAll(fv.elemChecks, elem) {
		return errs
	}
	value := elem.Interface()
	name := ""
	for _, rule := range fv.elemRules {
		if rule.fn(value, rule.param) {
			continue
		}
		if name == "" {
			name = fv.elemName(idx, key)
		}
		errs = append(errs, fv.fieldError(messages, rule, name, value))
	}
	return errs
}

// applyElemRulesNamed is applyElemRules with the label already built.
func (fv *fieldValidator) applyElemRulesNamed(messages map[string]string, elem reflect.Value, name string, errs []FieldError) []FieldError {
	if passesAll(fv.elemChecks, elem) {
		return errs
	}
	value := elem.Interface()
	for _, rule := range fv.elemRules {
		if !rule.fn(value, rule.param) {
			errs = append(errs, fv.fieldError(messages, rule, name, value))
		}
	}
	return errs
}

// formatsItself reports whether fmt's %v might call a method while
// formatting t: anything but a basic kind without its own formatting
// (structs and arrays format their fields, interfaces their dynamic value).
func formatsItself(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return t.Implements(formatterType) || t.Implements(stringerType) || t.Implements(errorType)
	}
	return true
}

func (fv *fieldValidator) elemName(idx int, key reflect.Value) string {
	if key.IsValid() {
		return fv.fieldName + "[" + fmt.Sprintf("%v", key.Interface()) + "]"
	}
	return fv.fieldName + "[" + strconv.Itoa(idx) + "]"
}

// passesAll reports whether a non-empty compiled chain accepts v. An empty
// chain means the rules could not all be compiled, so the boxed rules decide.
func passesAll(checks []func(reflect.Value) bool, v reflect.Value) bool {
	if len(checks) == 0 {
		return false
	}
	for _, check := range checks {
		if !check(v) {
			return false
		}
	}
	return true
}

// diveKind reports the collection kind `dive` will walk, or reflect.Invalid if
// the type is not a collection. Pointers are followed once.
func diveKind(t reflect.Type) reflect.Kind {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch k := t.Kind(); k {
	case reflect.Slice, reflect.Array, reflect.Map:
		return k
	}
	return reflect.Invalid
}

// warnedDiveTargets keeps the warning to one line per field.
var warnedDiveTargets sync.Map

func warnDiveOnNonCollection(typeName, fieldName, fieldType string) {
	key := typeName + "\x00" + fieldName
	if _, loaded := warnedDiveTargets.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	slog.Warn("kruda: `dive` on a field that is not a slice, array or map — the rules after it are ignored",
		"type", typeName, "field", fieldName, "field_type", fieldType)
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
	case reflect.Pointer, reflect.Interface:
		return !v.IsNil()
	default:
		return !v.IsZero()
	}
}

// compileMinMax is the unboxed mirror of validateMin/validateMax: for every
// (type, rule, param) it must return the same verdict the boxed originals
// would, or nil to take the original path. A check that wrongly passes would
// skip validation entirely, so any change to the min/max semantics here,
// there, or in the generated predicates (cmd/bindgen) must update all of
// them together. TestCompiledMinMaxParity is the drift guard: extend it when
// the semantics change, never narrow it to fit an implementation.
func compileMinMax(t reflect.Type, name, param string) func(reflect.Value) bool {
	if name != "min" && name != "max" {
		return nil
	}
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return nil
	}
	minimum := name == "min"
	switch t.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		if minimum {
			return func(v reflect.Value) bool { return float64(v.Len()) >= n }
		}
		return func(v reflect.Value) bool { return float64(v.Len()) <= n }
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ni, err := strconv.ParseInt(param, 10, 64); err == nil {
			if minimum {
				return func(v reflect.Value) bool { return v.Int() >= ni }
			}
			return func(v reflect.Value) bool { return v.Int() <= ni }
		}
		if minimum {
			return func(v reflect.Value) bool { return float64(v.Int()) >= n }
		}
		return func(v reflect.Value) bool { return float64(v.Int()) <= n }
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if nu, err := strconv.ParseUint(param, 10, 64); err == nil {
			if minimum {
				return func(v reflect.Value) bool { return v.Uint() >= nu }
			}
			return func(v reflect.Value) bool { return v.Uint() <= nu }
		}
		if minimum {
			return func(v reflect.Value) bool { return float64(v.Uint()) >= n }
		}
		return func(v reflect.Value) bool { return float64(v.Uint()) <= n }
	case reflect.Float32, reflect.Float64:
		if minimum {
			return func(v reflect.Value) bool { return v.Float() >= n }
		}
		return func(v reflect.Value) bool { return v.Float() <= n }
	}
	return nil
}

// compileRules compiles a whole chain, or returns nil when any rule cannot be
// compiled or is overridden, so the boxed rules run for every rule as before.
func compileRules(v *Validator, t reflect.Type, rules []ruleEntry) []func(reflect.Value) bool {
	var checks []func(reflect.Value) bool
	for _, rule := range rules {
		if v.customRules[rule.name] {
			return nil
		}
		check := compileRule(t, rule.name, rule.param)
		if check == nil {
			return nil
		}
		checks = append(checks, check)
	}
	return checks
}

// compileRule is the unboxed mirror of the built-in rule name. The same
// contract as compileMinMax applies: the same verdict as the boxed rule for
// every value and param, or nil. TestCompiledRuleParity guards it.
func compileRule(t reflect.Type, name, param string) func(reflect.Value) bool {
	switch name {
	case "min", "max":
		return compileMinMax(t, name, param)
	case "gt", "gte", "lt", "lte":
		return compileCompare(t, name, param)
	case "len":
		return compileLen(t, param)
	case "oneof":
		return compileOneOf(t, param)
	case "required":
		return compileRequired(t)
	}
	return nil
}

// diveElem is the element type `dive` walks, or nil for a non-collection.
func diveElem(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return t.Elem()
	}
	return nil
}

func compileCompare(t reflect.Type, name, param string) func(reflect.Value) bool {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return nil
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ni, err := strconv.ParseInt(param, 10, 64); err == nil {
			return orderedCheck(name, ni, reflect.Value.Int)
		}
		return orderedCheck(name, n, func(v reflect.Value) float64 { return float64(v.Int()) })
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if nu, err := strconv.ParseUint(param, 10, 64); err == nil {
			return orderedCheck(name, nu, reflect.Value.Uint)
		}
		return orderedCheck(name, n, func(v reflect.Value) float64 { return float64(v.Uint()) })
	case reflect.Float32, reflect.Float64:
		return orderedCheck(name, n, reflect.Value.Float)
	case reflect.String:
		return orderedCheck(name, n, func(v reflect.Value) float64 { return float64(v.Len()) })
	}
	return nil
}

func orderedCheck[N int64 | uint64 | float64](name string, p N, get func(reflect.Value) N) func(reflect.Value) bool {
	switch name {
	case "gt":
		return func(v reflect.Value) bool { return get(v) > p }
	case "gte":
		return func(v reflect.Value) bool { return get(v) >= p }
	case "lt":
		return func(v reflect.Value) bool { return get(v) < p }
	case "lte":
		return func(v reflect.Value) bool { return get(v) <= p }
	}
	return nil
}

func compileLen(t reflect.Type, param string) func(reflect.Value) bool {
	n, err := strconv.Atoi(param)
	if err != nil {
		return nil
	}
	switch t.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		return func(v reflect.Value) bool { return v.Len() == n }
	}
	return nil
}

var (
	formatterType = reflect.TypeOf((*fmt.Formatter)(nil)).Elem()
	stringerType  = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
	errorType     = reflect.TypeOf((*error)(nil)).Elem()
)

// compileOneOf covers string kinds only. validateOneOf compares fmt's %v
// text, which is the string itself unless the type formats itself.
func compileOneOf(t reflect.Type, param string) func(reflect.Value) bool {
	if t.Kind() != reflect.String || formatsItself(t) {
		return nil
	}
	opts := strings.Fields(param)
	return func(v reflect.Value) bool {
		s := v.String()
		for _, opt := range opts {
			if s == opt {
				return true
			}
		}
		return false
	}
}

// compileRequired skips interface fields: validateRequired sees the dynamic
// value there, not the field.
func compileRequired(t reflect.Type) func(reflect.Value) bool {
	switch t.Kind() {
	case reflect.Interface:
		return nil
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		return func(v reflect.Value) bool { return v.Len() > 0 }
	case reflect.Pointer:
		return func(v reflect.Value) bool { return !v.IsNil() }
	}
	return func(v reflect.Value) bool { return !v.IsZero() }
}

// validateMin is mirrored by compileMinMax (unboxed success path) and by the
// generated predicates in cmd/bindgen. See the contract there: all three
// must agree, or validation is silently skipped.
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

// validateMax is mirrored by compileMinMax (unboxed success path) and by the
// generated predicates in cmd/bindgen. See the contract there: all three
// must agree, or validation is silently skipped.
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

// stringValue returns the string held by value, including a named string type
// such as `type Email string`.
func stringValue(value any) (string, bool) {
	if s, ok := value.(string); ok {
		return s, true
	}
	if v := reflect.ValueOf(value); v.Kind() == reflect.String {
		return v.String(), true
	}
	return "", false
}

func validateEmail(value any, _ string) bool {
	s, ok := stringValue(value)
	if !ok || s == "" {
		return false
	}
	_, err := mail.ParseAddress(s)
	return err == nil
}

func validateURL(value any, _ string) bool {
	s, ok := stringValue(value)
	if !ok || s == "" {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func validateOneOf(value any, param string) bool {
	s := fmt.Sprintf("%v", value)
	for _, opt := range strings.Fields(param) {
		if s == opt {
			return true
		}
	}
	return false
}

// validateLen checks exact length (string/slice/map).
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
	s, ok := stringValue(value)
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

func validateAlpha(value any, _ string) bool {
	s, ok := stringValue(value)
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

func validateAlphanum(value any, _ string) bool {
	s, ok := stringValue(value)
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
	s, ok := stringValue(value)
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

func validateContains(value any, param string) bool {
	s, ok := stringValue(value)
	if !ok {
		return false
	}
	return strings.Contains(s, param)
}

func validateStartsWith(value any, param string) bool {
	s, ok := stringValue(value)
	if !ok {
		return false
	}
	return strings.HasPrefix(s, param)
}

func validateEndsWith(value any, param string) bool {
	s, ok := stringValue(value)
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
	return fu.Size <= maxBytes
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
