//go:build !linux

package wing

import (
	"fmt"

	"github.com/go-kruda/kruda/transport"
)

func (t *Transport) listenAndServePrefork(_ string, _ transport.Handler) error {
	return fmt.Errorf("wing: prefork not supported on this platform")
}
