package main

import "strconv"

// SerializeWorldJSON appends {"id":N,"randomNumber":N} to buf.
// Uses strconv.AppendInt for zero-reflection integer formatting.
func SerializeWorldJSON(buf []byte, w World) []byte {
	buf = append(buf, `{"id":`...)
	buf = strconv.AppendInt(buf, int64(w.ID), 10)
	buf = append(buf, `,"randomNumber":`...)
	buf = strconv.AppendInt(buf, int64(w.RandomNumber), 10)
	buf = append(buf, '}')
	return buf
}

// SerializeWorldsJSON appends [{"id":N,"randomNumber":N},...] to buf.
func SerializeWorldsJSON(buf []byte, worlds []World) []byte {
	buf = append(buf, '[')
	for i := range worlds {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = SerializeWorldJSON(buf, worlds[i])
	}
	buf = append(buf, ']')
	return buf
}

// SerializeMessageJSON appends {"message":"Hello, World!"} to buf.
func SerializeMessageJSON(buf []byte) []byte {
	buf = append(buf, `{"message":"Hello, World!"}`...)
	return buf
}

// HTML page fragments for the Fortunes template.
var (
	fortunePageStart = []byte("<!DOCTYPE html><html><head><title>Fortunes</title></head><body><table><tr><th>id</th><th>message</th></tr>")
	fortunePageEnd   = []byte("</table></body></html>")
	fortuneRowStart  = []byte("<tr><td>")
	fortuneRowMid    = []byte("</td><td>")
	fortuneRowEnd    = []byte("</td></tr>")
)

// SerializeFortunesHTML appends the complete Fortunes HTML page to buf.
// Builds the full page with DOCTYPE, html/head/title/body/table structure.
// Fortune messages are XSS-escaped via HTMLEscape.
func SerializeFortunesHTML(buf []byte, fortunes []Fortune) []byte {
	buf = append(buf, fortunePageStart...)
	for i := range fortunes {
		buf = append(buf, fortuneRowStart...)
		buf = strconv.AppendInt(buf, int64(fortunes[i].ID), 10)
		buf = append(buf, fortuneRowMid...)
		buf = HTMLEscape(buf, fortunes[i].Message)
		buf = append(buf, fortuneRowEnd...)
	}
	buf = append(buf, fortunePageEnd...)
	return buf
}

// HTMLEscape appends the HTML-escaped version of s to buf.
// Escapes the five HTML special characters:
//
//	'<' → &lt;   '>' → &gt;   '&' → &amp;   '"' → &#34;   '\'' → &#39;
//
// All other bytes (including UTF-8 multibyte continuation bytes) pass through
// unchanged, since none of the five special chars appear in continuation bytes.
// Does NOT use unsafe.Pointer — uses safe string indexing.
func HTMLEscape(buf []byte, s string) []byte {
	last := 0
	for i := 0; i < len(s); i++ {
		var esc string
		switch s[i] {
		case '<':
			esc = "&lt;"
		case '>':
			esc = "&gt;"
		case '&':
			esc = "&amp;"
		case '"':
			esc = "&#34;"
		case '\'':
			esc = "&#39;"
		default:
			continue
		}
		buf = append(buf, s[last:i]...)
		buf = append(buf, esc...)
		last = i + 1
	}
	buf = append(buf, s[last:]...)
	return buf
}
