//go:build kruda_stdjson || !cgo

package json

import (
	"bytes"
	"testing"
)

func TestMarshalString(t *testing.T) {
	data, err := Marshal("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `"hello"` {
		t.Fatalf("got %s, want %q", data, `"hello"`)
	}
}

func TestMarshalInt(t *testing.T) {
	data, err := Marshal(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "42" {
		t.Fatalf("got %s, want 42", data)
	}
}

func TestMarshalBool(t *testing.T) {
	data, err := Marshal(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "true" {
		t.Fatalf("got %s, want true", data)
	}
}

func TestMarshalNil(t *testing.T) {
	data, err := Marshal(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "null" {
		t.Fatalf("got %s, want null", data)
	}
}

func TestMarshalStruct(t *testing.T) {
	type user struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	data, err := Marshal(user{Name: "Alice", Age: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"name":"Alice","age":30}`
	if string(data) != expected {
		t.Fatalf("got %s, want %s", data, expected)
	}
}

func TestMarshalSlice(t *testing.T) {
	data, err := Marshal([]int{1, 2, 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "[1,2,3]" {
		t.Fatalf("got %s, want [1,2,3]", data)
	}
}

func TestMarshalErrorUnsupportedType(t *testing.T) {
	// Channels cannot be marshaled to JSON
	ch := make(chan int)
	_, err := Marshal(ch)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
}

func TestUnmarshalString(t *testing.T) {
	var s string
	if err := Unmarshal([]byte(`"world"`), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "world" {
		t.Fatalf("got %q, want %q", s, "world")
	}
}

func TestUnmarshalInt(t *testing.T) {
	var n int
	if err := Unmarshal([]byte("99"), &n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 99 {
		t.Fatalf("got %d, want 99", n)
	}
}

func TestUnmarshalStruct(t *testing.T) {
	type user struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	var u user
	if err := Unmarshal([]byte(`{"name":"Bob","age":25}`), &u); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Name != "Bob" || u.Age != 25 {
		t.Fatalf("got %+v, want {Name:Bob Age:25}", u)
	}
}

func TestUnmarshalInvalidJSON(t *testing.T) {
	var s string
	err := Unmarshal([]byte(`{invalid`), &s)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestUnmarshalNullIntoPointer(t *testing.T) {
	s := "initial"
	ptr := &s
	if err := Unmarshal([]byte("null"), &ptr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ptr != nil {
		t.Fatalf("expected nil pointer after unmarshaling null, got %v", ptr)
	}
}

func TestMarshalFloat(t *testing.T) {
	data, err := Marshal(3.14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "3.14" {
		t.Fatalf("got %s, want 3.14", data)
	}
}

func TestMarshalToBuffer_Struct(t *testing.T) {
	type item struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	buf := &bytes.Buffer{}
	err := MarshalToBuffer(buf, item{Name: "widget", Count: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"name":"widget","count":5}`
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestMarshalToBuffer_NoTrailingNewline(t *testing.T) {
	buf := &bytes.Buffer{}
	err := MarshalToBuffer(buf, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if got[len(got)-1] == '\n' {
		t.Fatal("output should not end with newline")
	}
	if got != `"hello"` {
		t.Fatalf("got %q, want %q", got, `"hello"`)
	}
}

func TestMarshalToBuffer_Nil(t *testing.T) {
	buf := &bytes.Buffer{}
	err := MarshalToBuffer(buf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "null" {
		t.Fatalf("got %q, want %q", buf.String(), "null")
	}
}

func TestMarshalToBuffer_Map(t *testing.T) {
	buf := &bytes.Buffer{}
	err := MarshalToBuffer(buf, map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != `{"a":1}` {
		t.Fatalf("got %q, want %q", buf.String(), `{"a":1}`)
	}
}

func TestMarshalToBuffer_Error(t *testing.T) {
	buf := &bytes.Buffer{}
	err := MarshalToBuffer(buf, make(chan int))
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestMarshalToBuffer_Slice(t *testing.T) {
	buf := &bytes.Buffer{}
	err := MarshalToBuffer(buf, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "[1,2,3]" {
		t.Fatalf("got %q, want %q", buf.String(), "[1,2,3]")
	}
}

func TestMarshalToBuffer_ReusesBuffer(t *testing.T) {
	buf := &bytes.Buffer{}
	// Write first
	_ = MarshalToBuffer(buf, "first")
	first := buf.String()
	// Reset and write second
	buf.Reset()
	_ = MarshalToBuffer(buf, "second")
	second := buf.String()
	if first != `"first"` || second != `"second"` {
		t.Fatalf("first=%q, second=%q", first, second)
	}
}
