package wing

// FeatherTable maps method+path to a Feather for per-route dispatch decisions.
// Lookup is two map reads (method → path → Feather), zero allocation.
//
// Usage:
//
//	wing.New(wing.Config{
//	    Feathers: map[string]Feather{
//	        "GET /plaintext": wing.Bolt,
//	        "GET /json":      wing.Flash,
//	        "GET /db":        wing.Arrow,
//	    },
//	})
type FeatherTable struct {
	routes   [mCount]map[string]Feather // indexed by method for O(1) first lookup
	prefixes [mCount][]prefixFeather    // param routes: prefix before ':'
	fallback map[string]Feather         // custom methods
	def      Feather
}

type prefixFeather struct {
	prefix  string
	feather Feather
}

// method index constants matching Wing's HTTP parser internMethod output.
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

// NewFeatherTable builds a FeatherTable from a user-friendly string map.
// Keys are "METHOD /path" (e.g. "GET /plaintext").
// Default feather is used for routes not in the table.
func NewFeatherTable(routes map[string]Feather, def Feather) FeatherTable {
	def.defaults()
	ft := FeatherTable{def: def}
	for key, f := range routes {
		f.defaults()
		method, path := splitKey(key)
		idx := methodIdx(method)
		if idx >= 0 {
			// Param route: store as prefix match
			if colonIdx := indexByte(path, ':'); colonIdx > 0 {
				ft.prefixes[idx] = append(ft.prefixes[idx], prefixFeather{
					prefix: path[:colonIdx], feather: f,
				})
			} else {
				if ft.routes[idx] == nil {
					ft.routes[idx] = make(map[string]Feather, 4)
				}
				ft.routes[idx][path] = f
			}
		} else {
			if ft.fallback == nil {
				ft.fallback = make(map[string]Feather, 2)
			}
			ft.fallback[key] = f
		}
	}
	return ft
}

// Lookup returns the Feather for the given method and path.
// Two map reads, zero allocation. Returns default if not found.
func (ft *FeatherTable) Lookup(method, path string) Feather {
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
				return pf.feather
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

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
