package observability

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kruda/kruda"
)

// doGET drives a GET through the app's net/http pipeline and returns the recorder.
func doGET(t *testing.T, app *kruda.App, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

// doGETStatus returns just the status code.
func doGETStatus(t *testing.T, app *kruda.App, path string) int {
	return doGET(t, app, path).Code
}
