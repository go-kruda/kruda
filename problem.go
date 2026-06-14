package kruda

import "encoding/json"

// ProblemDetails is an RFC 9457 (problem+json) error document, rendered for error
// responses when the app is created with WithProblemJSON(). Extensions are flattened
// to top-level members; reserved member names are ignored.
type ProblemDetails struct {
	Type       string
	Title      string
	Status     int
	Detail     string
	Instance   string
	Errors     []FieldError
	Extensions map[string]any
}

var problemReserved = map[string]struct{}{
	"type": {}, "title": {}, "status": {}, "detail": {}, "instance": {}, "errors": {},
}

// MarshalJSON flattens Extensions alongside the standard members. encoding/json keeps
// key order deterministic across builds, and error responses are not a hot path.
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

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
