package kruda

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
	routes   [mCount]map[string]Preset // indexed by method for O(1) first lookup
	prefixes [mCount][]prefixPreset    // param routes: prefix before ':'
	fallback map[string]Preset         // custom methods
	def      Preset
}

type prefixPreset struct {
	prefix  string
	preset Preset
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
	switch len(m) {
	case 3:
		if m[0] == 'G' {
			return mGet
		}
		if m[0] == 'P' && m[2] == 'T' {
			return mPut
		}
	case 4:
		if m[0] == 'P' {
			return mPost
		}
		if m[0] == 'H' {
			return mHead
		}
	case 5:
		return mPatch
	case 6:
		return mDelete
	}
	return -1
}

// NewPresetTable builds a PresetTable from a user-friendly string map.
// Keys are "METHOD /path" (e.g. "GET /plaintext").
// Default preset is used for routes not in the table.
func NewPresetTable(routes map[string]Preset, def Preset) PresetTable {
	def.defaults()
	ft := PresetTable{def: def}
	for key, f := range routes {
		f.defaults()
		f.explicit = true
		method, path := splitKey(key)
		idx := methodIdx(method)
		if idx >= 0 {
			// Param route: store as prefix match
			if colonIdx := wingIndexByte(path, ':'); colonIdx > 0 {
				f.handlers = nil
				f.path = ""
				f.pathClean = false
				ft.prefixes[idx] = append(ft.prefixes[idx], prefixPreset{
					prefix: path[:colonIdx], preset: f,
				})
			} else {
				if ft.routes[idx] == nil {
					ft.routes[idx] = make(map[string]Preset, 4)
				}
				f.path = path
				f.pathClean = !containsDotPercent(path)
				ft.routes[idx][path] = f
			}
		} else {
			if ft.fallback == nil {
				ft.fallback = make(map[string]Preset, 2)
			}
			ft.fallback[key] = f
		}
	}
	return ft
}

// Lookup returns the Preset for the given method and path.
// Two map reads, zero allocation. Returns default if not found.
func (ft *PresetTable) Lookup(method, path string) Preset {
	idx := methodIdx(method)
	if idx >= 0 {
		if m := ft.routes[idx]; m != nil {
			if f, ok := m[path]; ok {
				return f
			}
		}
		// Check param route prefixes
		for _, pf := range ft.prefixes[idx] {
			if len(path) >= len(pf.prefix) && path[:len(pf.prefix)] == pf.prefix {
				return pf.preset
			}
		}
	} else if ft.fallback != nil {
		key := method + " " + path
		if f, ok := ft.fallback[key]; ok {
			return f
		}
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
