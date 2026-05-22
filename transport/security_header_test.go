//go:build !race

package transport

import (
	"bytes"
	"testing"
)

func TestStaticResponseDoesNotEmitServerHeader(t *testing.T) {
	body := "ok-" + t.Name()
	resp := GetStaticResponseString(200, "text/plain", body)

	if bytes.Contains(resp, []byte("\r\nServer:")) {
		t.Fatal("static response should not emit Server header")
	}
}
