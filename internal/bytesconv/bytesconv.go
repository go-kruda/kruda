package bytesconv

import "unsafe"

// UnsafeString converts a byte slice to string without allocation.
// WARNING: The returned string shares memory with the byte slice.
// Do not modify the byte slice while the string is in use.
func UnsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// UnsafeBytes converts a string to byte slice without allocation.
// WARNING: The returned slice shares memory with the string.
// Do not modify the returned slice.
func UnsafeBytes(s string) []byte {
	if s == "" {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
