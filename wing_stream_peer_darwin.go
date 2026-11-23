//go:build darwin

package kruda

import (
	"context"
	"syscall"
)

func watchQueuedStreamPeer(syscall.RawConn, context.Context, <-chan struct{}) bool {
	return false
}
