package kruda

import (
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticAllowsDoubleDotsInsideFilename(t *testing.T) {
	app := New()
	app.StaticFS("/files", fstest.MapFS{"release..notes.txt": {Data: []byte("notes")}, "sub/file..txt": {Data: []byte("nested")}})
	for _, tc := range []struct {
		path, body string
		status     int
	}{{"/files/release..notes.txt", "notes", 200}, {"/files/sub/file..txt", "nested", 200}, {"/files/../secret.txt", "Forbidden", 403}, {"/files/sub/../../secret.txt", "Forbidden", 403}} {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
		if rec.Code != tc.status || rec.Body.String() != tc.body {
			t.Fatalf("%s got %d %q", tc.path, rec.Code, rec.Body.String())
		}
	}
}
