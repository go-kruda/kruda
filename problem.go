package kruda

import "encoding/json"

// ProblemDetails is an RFC 9457 (problem+json) error document. It is rendered for
// error responses when the app is created with WithProblemJSON(). The non-standard
// Extensions are flattened to top-level members; reserved member names are ignored.
type ProblemDetails struct {
	Type       string         // RFC 9457 "type" URI; defaults to "about:blank"
	Title      string         // RFC 9457 "title"
	Status     int            // RFC 9457 "status"
	Detail     string         // RFC 9457 "detail"; omitted when empty
	Instance   string         // RFC 9457 "instance"; omitted when empty
	Errors     []FieldError   // emitted as the "errors" member when non-empty
	Extensions map[string]any // arbitrary top-level extension members
}

// problemReserved holds the standard member names that Extensions may not override.
var problemReserved = map[string]struct{}{
	"type": {}, "title": {}, "status": {}, "detail": {}, "instance": {}, "errors": {},
}

// MarshalJSON renders the document with encoding/json. Error responses are not a hot
// path, so using the standard library here keeps key order deterministic on every
// build (Sonic and kruda_stdjson alike).
func (p ProblemDetails) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, 6+len(p.Extensions))
	for k, v := range p.Extensions {
		if _, reserved := problemReserved[k]; !reserved {
			m[k] = v
		}
	}
	m["type"] = orDefault(p.Type, "about:blank")
	m["title"] = p.Title
	m["status"] = p.Status
	if p.Detail != "" {
		m["detail"] = p.Detail
	}
	if p.Instance != "" {
		m["instance"] = p.Instance
	}
	if len(p.Errors) > 0 {
		m["errors"] = p.Errors
	}
	return json.Marshal(m)
}

// orDefault returns s, or def when s is empty.
func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
