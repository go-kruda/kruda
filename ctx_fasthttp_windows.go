//go:build windows

package kruda

// ctxFastHTTPFields is empty on Windows (no fasthttp support).
type ctxFastHTTPFields struct{}

func (c *Ctx) tryFastHTTPText(_ string) bool                        { return false }
func (c *Ctx) tryFastHTTPJSON(_ []byte) bool                        { return false }
func (c *Ctx) trySendBytesFastHTTP(_ []byte) bool                   { return false }
func (c *Ctx) trySendFastHTTP() bool                                { return false }
func (c *Ctx) trySetHeaderFastHTTP(_, _ string) bool                { return false }
func (c *Ctx) trySetHeaderBytesFastHTTP(_ string, _ []byte) bool    { return false }
func (c *Ctx) trySendBytesWithTypeFastHTTP(_ string, _ []byte) bool { return false }
func (c *Ctx) trySendBytesWithTypeBytesFastHTTP(_, _ []byte) bool   { return false }
func (c *Ctx) trySendStaticWithTypeBytesFastHTTP(_, _ []byte) bool  { return false }
func (c *Ctx) tryQueryFastHTTP(_ string) string                     { return "" }
func (c *Ctx) tryBodyBytesFastHTTP() ([]byte, bool)                 { return nil, false }
