package main

import (
	"bytes"
	"fmt"
)

func generateQueryCollection(out *bytes.Buffer, fields []field) map[string]string {
	names := make(map[string]string)
	var tags []string
	for _, f := range fields {
		if f.query == "" {
			continue
		}
		if _, exists := names[f.query]; exists {
			continue
		}
		name := fmt.Sprintf("query%d", len(tags))
		names[f.query] = name
		tags = append(tags, f.query)
		fmt.Fprintf(out, "var %s string\n", name)
	}
	if len(tags) == 0 {
		return names
	}
	fmt.Fprintln(out, "rawQuery, queryBound := c.RawQuery()")
	fmt.Fprintln(out, "if queryBound {")
	fmt.Fprintf(out, "remaining := %d\n", len(tags))
	for i := range tags {
		fmt.Fprintf(out, "var seen%d bool\n", i)
	}
	// Collect first, so conversion errors still follow struct field order.
	fmt.Fprintln(out, "for query := rawQuery; query != \"\" && remaining > 0; {")
	fmt.Fprintln(out, "var pair string\nif amp := strings.IndexByte(query, '&'); amp >= 0 { pair, query = query[:amp], query[amp+1:] } else { pair, query = query, \"\" }")
	fmt.Fprintln(out, "if eq := strings.IndexByte(pair, '='); eq >= 0 {\nswitch pair[:eq] {")
	for i, tag := range tags {
		fmt.Fprintf(out, "case %q:\nif !seen%d { %s = pair[eq+1:]; seen%d = true; remaining-- }\n", tag, i, names[tag], i)
	}
	fmt.Fprintln(out, "}\n}\n}\n}")
	return names
}
