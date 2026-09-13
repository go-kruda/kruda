//go:build goexperiment.jsonv2

package json

var invalidUTF8Golden = func() string {
	if EngineIsStdlib {
		return `"��"`
	}
	return `"\ufffd\ufffd"`
}()
