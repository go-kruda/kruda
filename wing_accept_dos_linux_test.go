//go:build linux

package kruda

import (
	"net/netip"
	"syscall"
	"testing"
	"unsafe"
)

func TestWingAccept_ParseSockaddrZeroAlloc(t *testing.T) {
	var rsa syscall.RawSockaddrAny
	p := (*syscall.RawSockaddrInet4)(unsafe.Pointer(&rsa))
	p.Family = syscall.AF_INET
	p.Addr = [4]byte{127, 0, 0, 1}
	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = parseRawSockaddr(&rsa)
	})
	if allocs != 0 {
		t.Fatalf("parseRawSockaddr must be zero-alloc for IPv4, got %.2f", allocs)
	}
}

func TestWingAccept_ParseSockaddrIPv4Value(t *testing.T) {
	var rsa syscall.RawSockaddrAny
	p := (*syscall.RawSockaddrInet4)(unsafe.Pointer(&rsa))
	p.Family = syscall.AF_INET
	p.Addr = [4]byte{10, 1, 2, 3}
	ip, ok := parseRawSockaddr(&rsa)
	if !ok || ip != netip.AddrFrom4([4]byte{10, 1, 2, 3}) {
		t.Fatalf("parseRawSockaddr v4 = %v,%v want 10.1.2.3", ip, ok)
	}
}
