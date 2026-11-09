//go:build linux || darwin

package kruda

import (
	"net"
	"testing"
)

func TestGetPeerAddrIPv6(t *testing.T) {
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	defer ln.Close()
	client, err := net.Dial("tcp6", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	raw, err := server.(*net.TCPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var addr string
	if err := raw.Control(func(fd uintptr) { addr = getPeerAddr(int32(fd)) }); err != nil {
		t.Fatal(err)
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host != "::1" {
		t.Fatalf("getPeerAddr = %q, host %q, err %v", addr, host, err)
	}
}
