//go:build windows

package kruda

import (
	"context"

	"github.com/go-kruda/kruda/transport"
)

// ctxFastHTTPFields is empty on Windows (no fasthttp support).
type ctxFastHTTPFields struct{}

func (c *Ctx) fastHTTPContext() context.Context                     { return nil }
func (c *Ctx) fastHTTPResponseWriter() transport.ResponseWriter     { return nil }
func (c *Ctx) fastHTTPRequest() transport.Request                   { return nil }
func (c *Ctx) tryFastHTTPText(_ string) bool                        { return false }
func (c *Ctx) tryFastHTTPJSONDirect(_ any) bool                     { return false }
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
