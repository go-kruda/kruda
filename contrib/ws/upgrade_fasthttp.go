package ws

import (
	"fmt"

	"github.com/go-kruda/kruda"
)

// upgradeFastHTTP performs WebSocket upgrade on fasthttp transport.
// fasthttp support requires importing github.com/valyala/fasthttp which adds
// an external dependency. For v1, we return a descriptive error directing users
// to use net/http transport for WebSocket. fasthttp WebSocket support will be
// added in a future version.
func (u *Upgrader) upgradeFastHTTP(c *kruda.Ctx, acceptKey string, handler func(*Conn)) error {
	return fmt.Errorf("ws: WebSocket upgrade on fasthttp transport is not yet supported in v1 — use net/http transport (kruda.NetHTTP())")
}
