//go:build darwin

package kruda

func submitIdleRecv(e engine, fd int32, buf []byte, offset int) {
	e.SubmitRecv(fd, buf, offset)
}
