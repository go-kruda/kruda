package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestParseTarget(t *testing.T) {
	got, err := parseTarget("http://127.0.0.1:3000/plaintext-handler?q=1")
	if err != nil {
		t.Fatalf("parseTarget returned error: %v", err)
	}
	if got.addr != "127.0.0.1:3000" {
		t.Fatalf("addr = %q", got.addr)
	}
	if got.hostHeader != "127.0.0.1:3000" {
		t.Fatalf("hostHeader = %q", got.hostHeader)
	}
	if got.requestURI != "/plaintext-handler?q=1" {
		t.Fatalf("requestURI = %q", got.requestURI)
	}
}

func TestReadResponseContentLength(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(
		"HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello" +
			"HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n",
	))

	status, err := readResponse(reader)
	if err != nil {
		t.Fatalf("first readResponse returned error: %v", err)
	}
	if status != 200 {
		t.Fatalf("first status = %d", status)
	}

	status, err = readResponse(reader)
	if err != nil {
		t.Fatalf("second readResponse returned error: %v", err)
	}
	if status != 204 {
		t.Fatalf("second status = %d", status)
	}
}

func TestReadResponseChunked(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n" +
			"5\r\nhello\r\n0\r\n\r\n",
	))

	status, err := readResponse(reader)
	if err != nil {
		t.Fatalf("readResponse returned error: %v", err)
	}
	if status != 200 {
		t.Fatalf("status = %d", status)
	}
}

func TestReadResponseRejectsUnboundedKeepAliveBody(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("HTTP/1.1 200 OK\r\nConnection: keep-alive\r\n\r\nhello"))

	if _, err := readResponse(reader); err == nil {
		t.Fatal("readResponse succeeded without Content-Length or chunked Transfer-Encoding")
	}
}
