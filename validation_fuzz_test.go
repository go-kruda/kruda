package kruda

import (
	"testing"
)

// FuzzValidateString fuzzes the built-in validation rule functions with
// arbitrary string values and arbitrary rule parameters.
//
// Approach: this fuzz targets the rule layer directly (one rule function per
// iteration, looked up by name from builtinRules()). Building a fresh struct
// per fuzz iteration to drive validate() is awkward — Go's testing.F doesn't
// support reflection-built types in seed corpora — and the rule functions are
// where any panic would actually originate. Driving them directly gives the
// fuzzer maximum coverage of the rule library.
//
// The rule name is selected by hashing the fuzz-supplied "rule" string into
// the set of registered builtin rules so every iteration exercises a real
// rule rather than wasting cycles on unknown-name early-returns.
//
// All built-in rules must return cleanly (true/false) for any input — never
// panic. Unknown rule params (e.g. min="abc") should produce false, not crash.
func FuzzValidateString(f *testing.F) {
	rules := builtinRules()
	// Stable order so the rule-pick is reproducible across runs.
	ruleNames := make([]string, 0, len(rules))
	for name := range rules {
		ruleNames = append(ruleNames, name)
	}
	// Sort for determinism — map iteration order is randomized in Go.
	for i := 1; i < len(ruleNames); i++ {
		for j := i; j > 0 && ruleNames[j] < ruleNames[j-1]; j-- {
			ruleNames[j], ruleNames[j-1] = ruleNames[j-1], ruleNames[j]
		}
	}

	// Seeds: (value, rule, param) tuples covering every built-in rule and
	// adversarial inputs (binary garbage, oversized values, malformed params).
	type seed struct {
		value, ruleName, param string
	}
	seeds := []seed{
		{"test", "required", ""},
		{"", "required", ""},
		{"a@b.com", "email", ""},
		{"not-an-email", "email", ""},
		{"https://example.com", "url", ""},
		{"://broken", "url", ""},
		{"123", "numeric", ""},
		{"-12", "numeric", ""},
		{"abc", "alpha", ""},
		{"abc123", "alphanum", ""},
		{"550e8400-e29b-41d4-a716-446655440000", "uuid", ""},
		{"not-a-uuid", "uuid", ""},
		{"hello", "min", "1"},
		{"hello", "min", "100"},
		{"hello", "max", "10"},
		{"hello", "len", "5"},
		{"hello", "gt", "0"},
		{"hello", "gte", "5"},
		{"hello", "lt", "10"},
		{"hello", "lte", "5"},
		{"hello", "contains", "ell"},
		{"hello", "startswith", "he"},
		{"hello", "endswith", "lo"},
		{"a", "oneof", "a b c"},
		{"x", "oneof", ""},
		{string([]byte{0xff, 0xfe, 0xfd}), "required", ""},
		{string(make([]byte, 4096)), "max", "10"},
		// Malformed rule params: rule should return false, not crash.
		{"5", "min", "not-a-number"},
		{"5", "min", "9999999999999999999999999"},
		{"5", "min", "-Inf"},
		{"5", "min", ""},
		{"x", "len", "not-an-int"},
		// Unknown rule name — handler must skip it gracefully.
		{"test", "totally-not-a-rule", "x"},
	}
	for _, s := range seeds {
		f.Add(s.value, s.ruleName, s.param)
	}

	f.Fuzz(func(t *testing.T, value, ruleName, param string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("validation rule panicked: rule=%q value=%q param=%q err=%v",
					ruleName, value, param, r)
			}
		}()

		// Try the fuzz-provided rule name verbatim first; if unknown, fall
		// back to a deterministic pick from the registered set so every
		// iteration still exercises real rule code.
		fn, ok := rules[ruleName]
		if !ok && len(ruleNames) > 0 {
			// fnv-1a-style fold of ruleName bytes → index. Cheap and stable.
			var h uint32 = 2166136261
			for i := 0; i < len(ruleName); i++ {
				h ^= uint32(ruleName[i])
				h *= 16777619
			}
			fn = rules[ruleNames[int(h)%len(ruleNames)]]
		}
		if fn == nil {
			return
		}

		// Run the rule against the string value.
		_ = fn(value, param)

		// Some rules have type-specific paths (int/uint/float/slice). Drive
		// those branches by passing typed values too — same param string.
		_ = fn(len(value), param)              // int branch
		_ = fn(uint(len(value)), param)        // uint branch
		_ = fn(float64(len(value))*1.5, param) // float branch
		_ = fn([]string{value}, param)         // slice branch
	})
}
