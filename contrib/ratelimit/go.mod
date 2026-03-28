module github.com/go-kruda/kruda/contrib/ratelimit

go 1.25.8

replace (
	github.com/go-kruda/kruda => ../../
	github.com/go-kruda/kruda/transport/wing => ../../transport/wing
)

require github.com/go-kruda/kruda v1.0.3

require (
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/go-kruda/kruda/transport/wing v0.0.0-00010101000000-000000000000 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.69.0 // indirect
	golang.org/x/arch v0.0.0-20210923205945-b76863e36670 // indirect
	golang.org/x/sys v0.39.0 // indirect
)
