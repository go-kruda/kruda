//go:build linux || darwin

package wing

// RawRequest provides low-level access to Wing's request data.
// Obtain via transport.Request.RawRequest():
//
//	if raw, ok := req.RawRequest().(wing.RawRequest); ok {
//	    fd := raw.Fd()
//	}
type RawRequest interface {
	RawMethod() string
	RawPath() []byte
	RawHeader(name string) []byte
	RawBody() []byte
	Fd() int32
	KeepAlive() bool
}

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
