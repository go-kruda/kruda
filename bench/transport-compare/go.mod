module github.com/go-kruda/kruda/bench/transport-compare

go 1.24.0

require (
	github.com/go-kruda/kruda v0.0.0
	github.com/go-kruda/kruda/transport/wing v0.0.0
)

replace (
	github.com/go-kruda/kruda => ../..
	github.com/go-kruda/kruda/transport/wing => ../../transport/wing
)
