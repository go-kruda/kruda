package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"testing/quick"
)

func TestPropertyWorldJSONRoundTrip(t *testing.T) {
	f := func(id, rn uint16) bool {
		// Constrain to valid TFB range [1, 10000]
		w := World{
			ID:           int32(id%10000) + 1,
			RandomNumber: int32(rn%10000) + 1,
		}
		buf := SerializeWorldJSON(nil, w)
		var got World
		if err := json.Unmarshal(buf, &got); err != nil {
			t.Logf("unmarshal error: %v, buf: %s", err, buf)
			return false
		}
		return got.ID == w.ID && got.RandomNumber == w.RandomNumber
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Error(err)
	}
}

func TestPropertyWorldSliceJSONRoundTrip(t *testing.T) {
	f := func(ids, rns []uint16) bool {
		// Clamp slice length to [0, 500]
		n := len(ids)
		if len(rns) < n {
			n = len(rns)
		}
		if n > 500 {
			n = 500
		}
		worlds := make([]World, n)
		for i := 0; i < n; i++ {
			worlds[i] = World{
				ID:           int32(ids[i]%10000) + 1,
				RandomNumber: int32(rns[i]%10000) + 1,
			}
		}
		buf := SerializeWorldsJSON(nil, worlds)
		var got []World
		if err := json.Unmarshal(buf, &got); err != nil {
			t.Logf("unmarshal error: %v, buf: %s", err, buf)
			return false
		}
		if len(got) != n {
			return false
		}
		for i := 0; i < n; i++ {
			if got[i] != worlds[i] {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

func TestPropertyXSSEscapeCompleteness(t *testing.T) {
	f := func(s string) bool {
		escaped := HTMLEscape(nil, s)
		result := string(escaped)

		// After escaping, no raw special characters should remain
		// outside of entity sequences.
		// Walk the result and check that < > & " ' only appear as part of entities.
		for i := 0; i < len(result); i++ {
			switch result[i] {
			case '<', '>', '"', '\'':
				// These must never appear raw in escaped output
				return false
			case '&':
				// '&' is allowed only as the start of a known entity
				rest := result[i:]
				if strings.HasPrefix(rest, "&lt;") ||
					strings.HasPrefix(rest, "&gt;") ||
					strings.HasPrefix(rest, "&amp;") ||
					strings.HasPrefix(rest, "&#34;") ||
					strings.HasPrefix(rest, "&#39;") {
					continue
				}
				// Raw '&' that isn't part of a known entity
				return false
			}
		}

		// Non-special bytes must be unchanged: for each byte in the input
		// that is NOT a special char, it must appear in the output at the
		// corresponding position (accounting for entity expansion).
		// We verify this by stripping entities and comparing non-special chars.
		stripped := result
		stripped = strings.ReplaceAll(stripped, "&lt;", "<")
		stripped = strings.ReplaceAll(stripped, "&gt;", ">")
		stripped = strings.ReplaceAll(stripped, "&amp;", "&")
		stripped = strings.ReplaceAll(stripped, "&#34;", "\"")
		stripped = strings.ReplaceAll(stripped, "&#39;", "'")
		return stripped == s
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Error(err)
	}
}

func TestPropertyHTMLStructureCompleteness(t *testing.T) {
	f := func(ids []int32, msgs []string) bool {
		// Build a non-empty Fortune slice of length min(len(ids), len(msgs)), capped at 100
		n := len(ids)
		if len(msgs) < n {
			n = len(msgs)
		}
		if n > 100 {
			n = 100
		}
		if n == 0 {
			// Property requires non-empty slice; skip trivially
			return true
		}

		fortunes := make([]Fortune, n)
		for i := 0; i < n; i++ {
			fortunes[i] = Fortune{ID: ids[i], Message: msgs[i]}
		}

		buf := SerializeFortunesHTML(nil, fortunes)
		html := string(buf)

		// Must contain DOCTYPE
		if !strings.Contains(html, "<!DOCTYPE html>") {
			return false
		}
		// Must contain structural elements
		for _, tag := range []string{"<html>", "<head>", "<title>", "</title>", "</head>", "<body>", "<table>", "</table>", "</body>", "</html>"} {
			if !strings.Contains(html, tag) {
				return false
			}
		}
		// Must contain header row with id and message columns
		if !strings.Contains(html, "<tr><th>id</th><th>message</th></tr>") {
			return false
		}
		// Must contain exactly N data rows (each fortune produces one <tr><td>...</td></tr>)
		// Total <tr> count = 1 (header) + N (data)
		trCount := bytes.Count(buf, []byte("<tr>"))
		if trCount != n+1 {
			return false
		}

		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}
