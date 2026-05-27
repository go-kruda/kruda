//go:build darwin

package kruda

func setSocketIncomingCPU(_ int, _ int) error {
	return nil
}

func pinCurrentThreadToCPU(_ int) error {
	return nil
}
