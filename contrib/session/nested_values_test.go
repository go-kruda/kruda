package session

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

func TestMemoryStore_NestedValuesAreOwned(t *testing.T) {
	for _, operation := range []string{"Save", "SaveExisting", "Regenerate"} {
		t.Run(operation, func(t *testing.T) {
			store := NewMemoryStore()
			defer store.Close()
			if err := store.Save("old", &SessionData{}, time.Hour); err != nil {
				t.Fatal(err)
			}
			data := &SessionData{
				Values: map[string]any{
					"roles":  []string{"reader"},
					"nested": map[string][]int{"counts": {1}},
					"array":  [1]map[string]string{{"name": "alice"}},
					"list":   []any{map[string]string{"name": "alice"}},
				},
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Hour),
			}
			id := "old"
			switch operation {
			case "Save":
				if err := store.Save(id, data, time.Hour); err != nil {
					t.Fatal(err)
				}
			case "SaveExisting":
				if saved, err := store.SaveExisting(id, data, time.Hour); err != nil || !saved {
					t.Fatalf("SaveExisting = %v, %v", saved, err)
				}
			case "Regenerate":
				id = "new"
				if moved, err := store.Regenerate("old", id, data, time.Hour); err != nil || !moved {
					t.Fatalf("Regenerate = %v, %v", moved, err)
				}
			}

			mutate := func(values map[string]any) {
				values["roles"].([]string)[0] = "admin"
				values["nested"].(map[string][]int)["counts"][0] = 2
				values["array"].([1]map[string]string)[0]["name"] = "mallory"
				values["list"].([]any)[0].(map[string]string)["name"] = "mallory"
			}
			mutate(data.Values)
			first, err := store.Get(id)
			if err != nil || first == nil {
				t.Fatalf("Get = %#v, %v", first, err)
			}
			want := map[string]any{
				"roles":  []string{"reader"},
				"nested": map[string][]int{"counts": {1}},
				"array":  [1]map[string]string{{"name": "alice"}},
				"list":   []any{map[string]string{"name": "alice"}},
			}
			if !reflect.DeepEqual(first.Values, want) {
				t.Fatalf("caller changed stored values: %#v", first.Values)
			}
			if first.CreatedAt != data.CreatedAt || first.ExpiresAt != data.ExpiresAt {
				t.Fatal("session timestamps changed")
			}
			second, err := store.Get(id)
			if err != nil || second == nil {
				t.Fatalf("second Get = %#v, %v", second, err)
			}
			mutate(first.Values)
			if !reflect.DeepEqual(second.Values, want) {
				t.Fatalf("one Get changed another snapshot: %#v", second.Values)
			}
			third, err := store.Get(id)
			if err != nil || third == nil || !reflect.DeepEqual(third.Values, want) {
				t.Fatalf("Get changed stored values: %#v, %v", third, err)
			}
		})
	}
}

func TestMemoryStore_NestedValueTypes(t *testing.T) {
	type userID uint64
	type roles []string
	type permissions map[userID]roles
	type roleArray [1]roles
	type customValue struct {
		Name    string
		private string
	}
	custom := &customValue{Name: "alice", private: "preserved"}
	values := map[string]any{
		"scalar":      userID(42),
		"slice":       roles{"reader"},
		"map":         permissions{42: {"reader"}},
		"array":       roleArray{{"reader"}},
		"struct":      *custom,
		"pointer":     custom,
		"nil":         nil,
		"nil_slice":   roles(nil),
		"nil_map":     permissions(nil),
		"nil_pointer": (*customValue)(nil),
		"empty_slice": roles{},
		"empty_map":   permissions{},
	}
	store := NewMemoryStore()
	defer store.Close()
	if err := store.Save("id", &SessionData{Values: values}, time.Hour); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("id")
	if err != nil || got == nil || !reflect.DeepEqual(got.Values, values) {
		t.Fatalf("value types or contents changed: %#v, %v", got, err)
	}
	if got.Values["pointer"] != custom {
		t.Fatal("custom pointer identity changed")
	}
	values["slice"].(roles)[0] = "admin"
	values["map"].(permissions)[42][0] = "admin"
	values["array"].(roleArray)[0][0] = "admin"
	if got.Values["slice"].(roles)[0] != "reader" || got.Values["map"].(permissions)[42][0] != "reader" || got.Values["array"].(roleArray)[0][0] != "reader" {
		t.Fatal("named containers share caller values")
	}
	for _, data := range []*SessionData{nil, {}} {
		if err := store.Save("nil", data, time.Hour); err != nil {
			t.Fatal(err)
		}
		got, err := store.Get("nil")
		if err != nil || !reflect.DeepEqual(got, data) {
			t.Fatalf("nil data or values changed: %#v, %v", got, err)
		}
	}
}

func TestMemoryStore_NestedValueCyclesAndAliases(t *testing.T) {
	values := make(map[string]any)
	nested := map[string]any{"name": "alice"}
	slice := make([]any, 2, 3)
	slice[0] = slice
	slice[1] = nested
	values["self"] = values
	values["map"] = nested
	values["map_alias"] = nested
	values["slice"] = slice
	values["slice_alias"] = slice
	values["short_slice"] = slice[:1]
	values["array"] = [1]any{slice}
	store := NewMemoryStore()
	defer store.Close()
	if err := store.Save("id", &SessionData{Values: values}, time.Hour); err != nil {
		t.Fatal(err)
	}
	nested["name"] = "caller"
	got, err := store.Get("id")
	if err != nil || got == nil {
		t.Fatalf("Get = %#v, %v", got, err)
	}
	gotMap := got.Values["map"].(map[string]any)
	gotSlice := got.Values["slice"].([]any)
	if gotMap["name"] != "alice" || len(gotSlice) != 2 || cap(gotSlice) != 2 {
		t.Fatal("stored map or visible slice changed")
	}
	got.Values["self"].(map[string]any)["request"] = true
	if got.Values["request"] != true || values["request"] != nil {
		t.Fatal("root map cycle was not copied")
	}
	gotMap["request"] = true
	if got.Values["map_alias"].(map[string]any)["request"] != true || gotSlice[1].(map[string]any)["request"] != true || nested["request"] != nil {
		t.Fatal("nested map aliases were not copied")
	}
	shortSlice := got.Values["short_slice"].([]any)
	if len(shortSlice) != 1 || cap(shortSlice) != 1 {
		t.Fatal("short slice includes hidden elements")
	}
	shortSlice[0] = "short"
	if _, ok := gotSlice[0].([]any); !ok {
		t.Fatal("different slice views share backing elements")
	}
	gotSlice[0].([]any)[1] = "request"
	if gotSlice[1] != "request" || got.Values["slice_alias"].([]any)[1] != "request" || got.Values["array"].([1]any)[0].([]any)[1] != "request" {
		t.Fatal("slice cycle or aliases were not copied")
	}
	again, err := store.Get("id")
	if err != nil || again == nil {
		t.Fatalf("second Get = %#v, %v", again, err)
	}
	againSlice := again.Values["slice"].([]any)
	if again.Values["request"] != nil || again.Values["map"].(map[string]any)["request"] != nil || againSlice[1].(map[string]any)["name"] != "alice" {
		t.Fatal("cyclic request values changed stored values")
	}
}

func TestMemoryStore_SaveDoesNotReadSliceTail(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	backing := []int{1, 0}
	data := &SessionData{Values: map[string]any{"visible": backing[:1]}}
	started := make(chan struct{})
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		close(started)
		for {
			select {
			case <-stop:
				return
			default:
				backing[1]++
			}
		}
	}()
	<-started
	defer func() {
		close(stop)
		<-done
	}()
	for i := 0; i < 100; i++ {
		if err := store.Save("id", data, time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.Get("id")
	if err != nil || got == nil || !reflect.DeepEqual(got.Values["visible"], []int{1}) {
		t.Fatalf("visible slice changed: %#v, %v", got, err)
	}
}

func TestConcurrentRequestsOwnNestedSessionValues(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	if err := store.Save("id", &SessionData{Values: map[string]any{
		"profile": map[string]any{"roles": []string{"reader"}},
	}}, time.Hour); err != nil {
		t.Fatal(err)
	}
	loaded := make(chan struct{}, 2)
	release := make(chan struct{})
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	handler := func(c *kruda.Ctx) error {
		sess := GetSession(c)
		roles := sess.Get("profile").(map[string]any)["roles"].([]string)
		loaded <- struct{}{}
		<-release
		for i := 0; i < 1000; i++ {
			roles[0] = c.Path()
		}
		sess.Set("updated", true)
		return c.Text(roles[0])
	}
	app.Post("/left", handler)
	app.Post("/right", handler)
	app.Compile()
	type result struct {
		path   string
		status int
		body   string
	}
	results := make(chan result, 2)
	for _, path := range []string{"/left", "/right"} {
		go func() {
			resp := serveSessionRequest(app, http.MethodPost, path, &http.Cookie{Name: "_session", Value: "id"})
			results <- result{path, resp.Code, resp.Body.String()}
		}()
	}
	<-loaded
	<-loaded
	close(release)
	for i := 0; i < 2; i++ {
		got := <-results
		if got.status != http.StatusOK || got.body != got.path {
			t.Errorf("%s: status=%d body=%q", got.path, got.status, got.body)
		}
	}
}
