package ws

import "github.com/go-kruda/kruda"

// upgradeNetHTTP performs the WebSocket upgrade on the net/http transport via
// the standard http.Hijacker. The hijack logic is shared with the Wing path.
func (u *Upgrader) upgradeNetHTTP(c *kruda.Ctx, acceptKey string, handler func(*Conn)) error {
	return upgradeHijacker(c, acceptKey, handler, u.Config)
}
