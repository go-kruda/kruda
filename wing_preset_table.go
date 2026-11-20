package kruda

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
)

// PresetTable maps method+path to a Preset for per-route dispatch decisions.
// Lookup is two map reads (method → path → Preset), zero allocation.
//
// Usage:
//
//	NewWingTransport(WingConfig{
//	    Presets: map[string]Preset{
//	        "GET /plaintext": Bolt,
//	        "GET /json":      Bolt,
//	        "GET /db":        Arrow,
//	    },
//	})
type PresetTable struct {
	std     [mCount]methodPresets     // indexed by method for O(1) first lookup
	custom  map[string]*methodPresets // OPTIONS and other methods
	def     Preset
	defHead Preset // def without a static body, for HEAD
	// cleanPaths mirrors App PathTraversal: the router sees the cleaned path,
	// so the lookup must too.
	cleanPaths bool
}

// methodPresets holds one method's routes: exact paths in a map, param and
// wildcard patterns in a list ordered most specific first.
type methodPresets struct {
	routes   map[string]Preset
	prefixes []prefixPreset
}

func (mp *methodPresets) add(path string, f Preset) {
	if isDynamic(path) {
		f.handlers = nil
		f.path = ""
		f.pathClean = false
		mp.prefixes = append(mp.prefixes, newPrefixPreset(path, f))
		return
	}
	if mp.routes == nil {
		mp.routes = make(map[string]Preset, 4)
	}
	mp.routes[path] = f
}

func isDynamic(path string) bool {
	return wingIndexByte(path, ':') > 0 || wingIndexByte(path, '*') > 0
}

type prefixPreset struct {
	pattern string
	segs    []string         // pattern split on '/', without the leading empty segment
	rx      []*regexp.Regexp // per segment; set only for a ":name<re>" constraint
	preset  Preset
}

func newPrefixPreset(path string, f Preset) prefixPreset {
	pf := prefixPreset{pattern: path, segs: strings.Split(strings.TrimPrefix(path, "/"), "/"), preset: f}
	for i, seg := range pf.segs {
		if rx := segRegex(seg); rx != nil {
			if pf.rx == nil {
				pf.rx = make([]*regexp.Regexp, len(pf.segs))
			}
			pf.rx[i] = rx
		}
	}
	return pf
}

// segRegex compiles a ":name<re>" constraint the way the router does. An
// invalid one yields nil; the router has already rejected it at registration.
func segRegex(seg string) *regexp.Regexp {
	if seg == "" || seg[0] != ':' {
		return nil
	}
	raw := strings.TrimSuffix(seg[1:], "?")
	start, end := strings.IndexByte(raw, '<'), strings.IndexByte(raw, '>')
	if start < 0 || end < start {
		return nil
	}
	rx, err := regexp.Compile("^" + raw[start+1:end] + "$")
	if err != nil {
		return nil
	}
	return rx
}

// matches reports whether path fits the route pattern segment by segment:
// ":name" takes one non-empty segment (":name?" may also be absent at the end),
// "*name" takes the rest, anything else must be equal. A regex constraint on
// a param must also hold, as the router skips the route otherwise.
func (pf *prefixPreset) matches(path string) bool {
	if len(path) == 0 || path[0] != '/' {
		return false
	}
	rest := path[1:]
	for i, seg := range pf.segs {
		if seg != "" && seg[0] == '*' {
			return true
		}
		end := strings.IndexByte(rest, '/')
		var part string
		if end < 0 {
			part, end = rest, len(rest)
		} else {
			part = rest[:end]
		}
		last := i == len(pf.segs)-1
		if seg != "" && seg[0] == ':' {
			if part == "" {
				return last && strings.HasSuffix(seg, "?") && end == len(rest)
			}
			if pf.rx != nil && pf.rx[i] != nil && !pf.rx[i].MatchString(part) {
				return false
			}
		} else if part != seg {
			return false
		}
		if last {
			return end == len(rest)
		}
		if end == len(rest) {
			// Path ended; only an optional param or a wildcard may follow.
			next := pf.segs[i+1]
			return i+1 == len(pf.segs)-1 && next != "" && (next[0] == '*' || (next[0] == ':' && strings.HasSuffix(next, "?")))
		}
		rest = rest[end+1:]
	}
	return rest == ""
}

// segRank orders segment kinds the way the router prefers them. A regex param
// ranks before a plain one: where both fit a value, the constrained route is
// the one meant for it.
func segRank(seg string) int {
	switch {
	case seg != "" && seg[0] == ':' && strings.IndexByte(seg, '<') > 0:
		return 1
	case seg != "" && seg[0] == ':':
		return 2
	case seg != "" && seg[0] == '*':
		return 3
	}
	return 0
}

// morePrecise orders patterns so the first match is the most specific one,
// and so the order does not depend on map iteration.
func morePrecise(a, b []string, pa, pb string) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if ra, rb := segRank(a[i]), segRank(b[i]); ra != rb {
			return ra < rb
		}
	}
	if len(a) != len(b) {
		return len(a) > len(b)
	}
	return pa < pb
}

// method index constants matching Wing's HTTP parser wingInternMethod output.
const (
	mGet = iota
	mPost
	mPut
	mDelete
	mPatch
	mHead
	mCount
)

// methodIdx returns array index for standard methods. -1 for unknown.
func methodIdx(m string) int {
	switch m {
	case "GET":
		return mGet
	case "POST":
		return mPost
	case "PUT":
		return mPut
	case "DELETE":
		return mDelete
	case "PATCH":
		return mPatch
	case "HEAD":
		return mHead
	}
	return -1
}

// NewPresetTable builds a PresetTable from a user-friendly string map.
// Keys are "METHOD /path" (e.g. "GET /plaintext").
// Default preset is used for routes not in the table.
func NewPresetTable(routes map[string]Preset, def Preset) PresetTable {
	def.defaults()
	ft := PresetTable{def: def, defHead: def}
	ft.defHead.StaticResponse = staticHeadersOnly(def.StaticResponse)
	// Routes without a preset matter only where a param or wildcard preset of
	// the same method could otherwise answer for them.
	dynamic := map[string]bool{}
	for key, f := range routes {
		if method, path := splitKey(key); !f.inherit && isDynamic(path) {
			dynamic[method] = true
		}
	}
	for key, f := range routes {
		method, path := splitKey(key)
		if f.inherit {
			if !dynamic[method] {
				continue
			}
			f = ft.def
			if method == "HEAD" {
				f = ft.defHead
			}
			ft.methodFor(method).add(path, f)
			continue
		}
		f.defaults()
		f.explicit = true
		if method == "HEAD" {
			f.StaticResponse = staticHeadersOnly(f.StaticResponse)
		}
		if !isDynamic(path) {
			f.path = path
			f.pathClean = !containsDotPercent(path)
		}
		ft.methodFor(method).add(path, f)
	}
	sortPrefixes := func(ps []prefixPreset) {
		sort.Slice(ps, func(a, b int) bool {
			return morePrecise(ps[a].segs, ps[b].segs, ps[a].pattern, ps[b].pattern)
		})
	}
	for i := range ft.std {
		sortPrefixes(ft.std[i].prefixes)
	}
	for _, mp := range ft.custom {
		sortPrefixes(mp.prefixes)
	}
	return ft
}

func (ft *PresetTable) methodFor(method string) *methodPresets {
	if idx := methodIdx(method); idx >= 0 {
		return &ft.std[idx]
	}
	if ft.custom == nil {
		ft.custom = make(map[string]*methodPresets, 2)
	}
	mp := ft.custom[method]
	if mp == nil {
		mp = &methodPresets{}
		ft.custom[method] = mp
	}
	return mp
}

// Lookup returns the Preset for the given method and path, or the default.
// Zero allocation.
func (ft *PresetTable) Lookup(method, path string) Preset {
	if ft.cleanPaths && containsDotPercent(path) {
		if p, err := cleanPath(path); err == nil {
			path = p
		}
	}
	idx := methodIdx(method)
	var mp *methodPresets
	if idx >= 0 {
		mp = &ft.std[idx]
	} else {
		mp = ft.custom[method]
	}
	if mp != nil {
		if f, ok := mp.routes[path]; ok {
			return f
		}
		for i := range mp.prefixes {
			if pf := &mp.prefixes[i]; pf.matches(path) {
				return pf.preset
			}
		}
	}
	if idx == mHead {
		return ft.defHead
	}
	return ft.def
}

// splitKey splits "GET /plaintext" into ("GET", "/plaintext").
func splitKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == ' ' {
			return key[:i], key[i+1:]
		}
	}
	return key, "/"
}

func wingIndexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// staticHeadersOnly cuts a prebuilt response down to its headers, so a HEAD
// route sends the length a GET would and no body. The capped slice keeps the
// shared cached bytes from being appended to.
func staticHeadersOnly(resp []byte) []byte {
	if i := bytes.Index(resp, []byte("\r\n\r\n")); i >= 0 {
		return resp[: i+4 : i+4]
	}
	return resp
}
