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
	return fmt.Errorf("ws: WebSocket upgrade requires net/http transport — add kruda.NetHTTP() option to kruda.New() (macOS defaults to fasthttp which does not support WebSocket)")
}
