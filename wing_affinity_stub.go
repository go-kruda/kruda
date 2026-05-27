//go:build darwin

package kruda

func pinCurrentThreadToCPU(_ int) error {
	return nil
}
