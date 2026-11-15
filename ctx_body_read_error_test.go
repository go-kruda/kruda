package kruda

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type errorBodyReader struct {
	*strings.Reader
	err      error
	withData bool
}

func (r *errorBodyReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF || r.withData && r.Len() == 0 {
		return n, r.err
	}
	return n, err
}

func TestServeHTTPBodyReadError(t *testing.T) {
	const payload = `{"name":"partial"}`
	readErr := errors.New("body read failed")
	for _, tc := range []struct {
		name          string
		payload       string
		readErr       error
		withData      bool
		contentLength int64
		limit         int
		wantBody      string
		wantErr       error
		wantStatus    int
	}{
		{name: "clean EOF after data", payload: payload, readErr: io.EOF, contentLength: -1, wantBody: payload, wantStatus: http.StatusOK},
		{name: "clean EOF with data", payload: payload, readErr: io.EOF, withData: true, contentLength: -1, wantBody: payload, wantStatus: http.StatusOK},
		{name: "unexpected EOF after data", payload: payload, readErr: io.ErrUnexpectedEOF, contentLength: -1, wantBody: payload, wantErr: io.ErrUnexpectedEOF, wantStatus: http.StatusBadRequest},
		{name: "unexpected EOF with data", payload: payload, readErr: io.ErrUnexpectedEOF, withData: true, contentLength: -1, wantBody: payload, wantErr: io.ErrUnexpectedEOF, wantStatus: http.StatusBadRequest},
		{name: "reader error with data", payload: payload, readErr: readErr, withData: true, contentLength: int64(len(payload)), wantBody: payload, wantErr: readErr, wantStatus: http.StatusBadRequest},
		{name: "reader error without data", readErr: readErr, contentLength: -1, wantErr: readErr, wantStatus: http.StatusBadRequest},
		{name: "known length exceeds limit", payload: payload, readErr: io.ErrUnexpectedEOF, withData: true, contentLength: int64(len(payload)), limit: len(payload) - 1, wantErr: ErrBodyTooLarge, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "stream exceeds limit", payload: payload, readErr: io.ErrUnexpectedEOF, withData: true, contentLength: -1, limit: len(payload) - 1, wantErr: ErrBodyTooLarge, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "body at limit", payload: payload, readErr: io.EOF, withData: true, contentLength: -1, limit: len(payload), wantBody: payload, wantStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := New(WithBodyLimit(tc.limit))
			called := false
			app.Post("/body", func(c *Ctx) error {
				called = true
				for i := 0; i < 2; i++ {
					body, err := c.BodyBytes()
					if string(body) != tc.wantBody || err != tc.wantErr {
						t.Errorf("BodyBytes call %d = %q, %v; want %q, %v", i+1, body, err, tc.wantBody, tc.wantErr)
					}
				}
				var input struct {
					Name string `json:"name"`
				}
				if err := c.Bind(&input); err != nil {
					return err
				}
				return c.Text(input.Name)
			})
			app.Compile()

			body := &errorBodyReader{Reader: strings.NewReader(tc.payload), err: tc.readErr, withData: tc.withData}
			req := httptest.NewRequest(http.MethodPost, "/body", body)
			req.ContentLength = tc.contentLength
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			app.ServeHTTP(resp, req)

			if !called {
				t.Fatal("body handler was not called")
			}
			if resp.Code != tc.wantStatus {
				t.Errorf("Bind response status = %d; want %d, body = %s", resp.Code, tc.wantStatus, resp.Body.String())
			}
			if tc.wantStatus == http.StatusOK && resp.Body.String() != "partial" {
				t.Errorf("Bind response body = %q; want partial", resp.Body.String())
			}
		})
	}
}
