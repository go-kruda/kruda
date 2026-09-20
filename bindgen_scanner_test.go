//go:build linux || darwin

package kruda

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// wingScanCtx builds a Ctx backed by a real wingRequest carrying a raw query
// string, so generated binders take the single-pass scanner path
// (queryBound=true) instead of the per-field c.Query fallback. The type
// assertion pins that premise: if it ever fails, this harness is silently
// testing the fallback and must be fixed, not the binder.
func wingScanCtx(t *testing.T, app *App, rawQuery string) *Ctx {
	t.Helper()
	resp := acquireResponse()
	t.Cleanup(func() { releaseResponse(resp) })
	req := &wingRequest{method: "GET", path: "/users/42", query: rawQuery, keepAlive: true}
	c := newCtx(app)
	c.resetWing(resp, req)
	if _, ok := c.RawQuery(); !ok {
		t.Fatal("wingScanCtx: RawQuery declined, scanner path untested")
	}
	return c
}

func assertRawQueryParity(t *testing.T, parser *inputParser, app *App, rawQuery string) {
	t.Helper()
	want, wantErr := parser.parse(wingScanCtx(t, app, rawQuery))
	got, gotErr := mustBindGeneratedBinder(t)(wingScanCtx(t, app, rawQuery))
	if (wantErr == nil) != (gotErr == nil) {
		t.Fatalf("query %q: normal err=%v generated err=%v", rawQuery, wantErr, gotErr)
	}
	if wantErr != nil {
		var wantKe, gotKe *KrudaError
		if !errors.As(wantErr, &wantKe) || !errors.As(gotErr, &gotKe) {
			t.Fatalf("query %q: normal err=%v generated err=%v", rawQuery, wantErr, gotErr)
		}
		if wantKe.Code != gotKe.Code || wantKe.Message != gotKe.Message {
			t.Fatalf("query %q: normal %d %q, generated %d %q",
				rawQuery, wantKe.Code, wantKe.Message, gotKe.Code, gotKe.Message)
		}
		return
	}
	gotInput := got.Interface().(generatedBinderInput)
	wantInput := want.Interface().(generatedBinderInput)
	if gotInput.ID != wantInput.ID || gotInput.Page != wantInput.Page ||
		gotInput.Active != wantInput.Active || gotInput.Name != wantInput.Name {
		t.Fatalf("query %q: generated=%+v normal=%+v", rawQuery, gotInput, wantInput)
	}
	if !(math.IsNaN(gotInput.Score) && math.IsNaN(wantInput.Score)) &&
		math.Float64bits(gotInput.Score) != math.Float64bits(wantInput.Score) {
		t.Fatalf("query %q: generated score=%v (%x), normal=%v (%x)",
			rawQuery, gotInput.Score, math.Float64bits(gotInput.Score),
			wantInput.Score, math.Float64bits(wantInput.Score))
	}
}

func TestGeneratedBinderRawQueryParity(t *testing.T) {
	app := New()
	parser := buildInputParser[generatedBinderInput]()
	var unknowns []string
	for i := 0; i < 30; i++ {
		unknowns = append(unknowns, "u"+string(rune('a'+i/26))+string(rune('a'+i%26))+"=0")
	}
	trailing := strings.Join(unknowns, "&")
	queries := []string{
		"",
		"id=8",
		"id=8&page=2&active=false&score=2.5&name=bob",
		// Duplicates: first equals-bearing occurrence wins.
		"id=1&id=2",
		"id=&id=2",
		"id=2&id=",
		// Bare tokens are ignored.
		"flag&id=3",
		"id=3&flag",
		"flag",
		// Empty values behave as absent.
		"id=&page=2",
		// Unknown keys and stray separators.
		"zzz=1&id=4&yyy=2",
		"&id=5&",
		"&&&",
		"=",
		"&=&",
		// '=' inside values belongs to the value.
		"name=a=b=c",
		"name==x",
		// Wing never unescapes: raw bytes pass through on both paths.
		"name=a%20b",
		"name=a+b",
		"a%20b=1&id=6",
		"id=%38",
		"id=1%32",
		// ';' is not a separator on either path.
		"id=1;page=2",
		// Prefix keys must not collide.
		"idx=9&id=7",
		"id=7&idx=9",
		"page2=5&page=3",
		// Unicode keys and values.
		"name=ไทย&id=10",
		"ไทย=1&id=11",
		// Keys are case-sensitive.
		"ID=12&id=13",
		// Late keys after many unknowns: early-stop must not matter.
		trailing + "&id=14&page=4&active=false&score=0.5&name=late",
		// Targets first, trailing unknowns: the original v1 regression shape.
		"id=15&page=5&active=true&score=3.5&name=early&" + trailing,
		// Raw control bytes pass through identically.
		"name=x\x00y",
		"name=\xff",
		// Invalid values still error identically.
		"id=bad",
		"id=1&id=bad",
		"id=bad&id=1",
		"page=1.5",
		"active=yes",
		"score=1x",
		"page=bad&id=also-bad",
	}
	for _, q := range queries {
		assertRawQueryParity(t, parser, app, q)
	}
}

// TestGeneratedBinderScannerSemantics pins the scanner's observable contract
// with explicit values, proving the parity test above exercises the
// queryBound=true path rather than vacuously agreeing on the fallback.
func TestGeneratedBinderScannerSemantics(t *testing.T) {
	app := New()
	for _, tt := range []struct {
		query string
		want  generatedBinderInput
	}{
		{"id=1&id=2", generatedBinderInput{ID: 1, Page: 1, Active: true, Score: 1.5, Name: "guest"}},
		{"flag&id=3", generatedBinderInput{ID: 3, Page: 1, Active: true, Score: 1.5, Name: "guest"}},
		{"id=&page=2", generatedBinderInput{ID: 7, Page: 2, Active: true, Score: 1.5, Name: "guest"}},
		{"name=a%20b", generatedBinderInput{ID: 7, Page: 1, Active: true, Score: 1.5, Name: "a%20b"}},
	} {
		got, err := mustBindGeneratedBinder(t)(wingScanCtx(t, app, tt.query))
		if err != nil {
			t.Fatalf("query %q: unexpected error %v", tt.query, err)
		}
		if input := got.Interface().(generatedBinderInput); input != tt.want {
			t.Fatalf("query %q: got %+v, want %+v", tt.query, input, tt.want)
		}
	}
}
