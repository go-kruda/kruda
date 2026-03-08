//go:build kruda_stdjson || !cgo

package json

import (
	"reflect"
	"testing"
	"testing/quick"
)

func TestPropertyJSONRoundTripString(t *testing.T) {
	f := func(s string) bool {
		data, err := Marshal(s)
		if err != nil {
			return false
		}
		var got string
		if err := Unmarshal(data, &got); err != nil {
			return false
		}
		return got == s
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("round trip failed for string: %v", err)
	}
}

func TestPropertyJSONRoundTripInt(t *testing.T) {
	f := func(n int) bool {
		data, err := Marshal(n)
		if err != nil {
			return false
		}
		var got int
		if err := Unmarshal(data, &got); err != nil {
			return false
		}
		return got == n
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("round trip failed for int: %v", err)
	}
}

func TestPropertyJSONRoundTripFloat64(t *testing.T) {
	f := func(n float64) bool {
		// Skip NaN and Inf — not representable in JSON
		if n != n { // NaN
			return true
		}
		if n > 1e308 || n < -1e308 { // Inf
			return true
		}
		data, err := Marshal(n)
		if err != nil {
			return false
		}
		var got float64
		if err := Unmarshal(data, &got); err != nil {
			return false
		}
		return got == n
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("round trip failed for float64: %v", err)
	}
}

func TestPropertyJSONRoundTripBool(t *testing.T) {
	f := func(b bool) bool {
		data, err := Marshal(b)
		if err != nil {
			return false
		}
		var got bool
		if err := Unmarshal(data, &got); err != nil {
			return false
		}
		return got == b
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("round trip failed for bool: %v", err)
	}
}

type testStruct struct {
	Name   string  `json:"name"`
	Age    int     `json:"age"`
	Score  float64 `json:"score"`
	Active bool    `json:"active"`
}

func TestPropertyJSONRoundTripStruct(t *testing.T) {
	f := func(name string, age int, score float64, active bool) bool {
		// Skip non-JSON-representable floats
		if score != score || score > 1e308 || score < -1e308 {
			return true
		}
		original := testStruct{Name: name, Age: age, Score: score, Active: active}
		data, err := Marshal(original)
		if err != nil {
			return false
		}
		var got testStruct
		if err := Unmarshal(data, &got); err != nil {
			return false
		}
		return reflect.DeepEqual(original, got)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("round trip failed for struct: %v", err)
	}
}
