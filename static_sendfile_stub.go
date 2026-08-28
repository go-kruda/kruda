//go:build !linux && !darwin

package kruda

import "os"

func duplicateStaticFile(*os.File) (int32, bool) {
	return 0, false
}
