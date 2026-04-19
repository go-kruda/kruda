//go:build linux || darwin

package kruda

// RawRequest is declared in wing_types_shared.go; this file holds the
// linux/darwin implementation on *wingRequest.
var _ RawRequest = (*wingRequest)(nil)

func (r *wingRequest) RawMethod() string { return r.method }
func (r *wingRequest) RawPath() []byte   { return []byte(r.path) }
func (r *wingRequest) RawBody() []byte   { return r.body }
func (r *wingRequest) Fd() int32         { return r.fd }
func (r *wingRequest) KeepAlive() bool   { return r.keepAlive }
func (r *wingRequest) RawHeader(name string) []byte {
	v := (*wingRequest)(r).Header(name)
	if v == "" {
		return nil
	}
	return []byte(v)
}
