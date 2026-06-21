package kruda

import (
	"reflect"
	"testing"
)

// TestResourceParseID_IntOverflowErrors guards the error path of the
// width-aware integer parse. The (MaxInt32, MaxInt64] silent-truncation it
// prevents is reachable only on 32-bit builds; here a value beyond int64
// exercises the ParseInt/ParseUint range error on any arch.
func TestResourceParseID_IntOverflowErrors(t *testing.T) {
	if _, err := resourceParseID[int64]("99999999999999999999999"); err == nil {
		t.Fatal("expected range error for value beyond int64")
	}
	if _, err := resourceParseID[uint64]("99999999999999999999999"); err == nil {
		t.Fatal("expected range error for value beyond uint64")
	}
}

// TestResource_IntIDOverflow400 confirms an out-of-range id surfaces as a clean
// 400 end-to-end, never a silently truncated wrong-ID 200.
func TestResource_IntIDOverflow400(t *testing.T) {
	app := New()
	Resource[namedIDItem, NamedID](app, "/things", namedIDService{})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/things/99999999999999999999999"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400 for out-of-range id\nbody: %s", resp.statusCode, resp.body)
	}
}

// TestResourceParseID_AllSupportedKinds asserts the registration gate
// (resourceIDKindSupported) and the parser agree: every admitted kind parses a
// valid sample, and rejected kinds are not silently parseable.
func TestResourceParseID_AllSupportedKinds(t *testing.T) {
	if v, err := resourceParseID[string]("x"); err != nil || v != "x" {
		t.Errorf("string: got (%q,%v)", v, err)
	}
	if v, err := resourceParseID[int]("7"); err != nil || v != 7 {
		t.Errorf("int: got (%d,%v)", v, err)
	}
	if v, err := resourceParseID[int64]("7"); err != nil || v != 7 {
		t.Errorf("int64: got (%d,%v)", v, err)
	}
	if v, err := resourceParseID[uint]("7"); err != nil || v != 7 {
		t.Errorf("uint: got (%d,%v)", v, err)
	}
	if v, err := resourceParseID[uint64]("7"); err != nil || v != 7 {
		t.Errorf("uint64: got (%d,%v)", v, err)
	}

	if !resourceIDKindSupported(reflect.String) || !resourceIDKindSupported(reflect.Int) ||
		!resourceIDKindSupported(reflect.Int64) || !resourceIDKindSupported(reflect.Uint) ||
		!resourceIDKindSupported(reflect.Uint64) {
		t.Error("gate predicate rejects a kind the parser handles")
	}
	if resourceIDKindSupported(reflect.Int32) || resourceIDKindSupported(reflect.Float64) ||
		resourceIDKindSupported(reflect.Bool) {
		t.Error("gate predicate admits a kind the parser does not support")
	}
}
