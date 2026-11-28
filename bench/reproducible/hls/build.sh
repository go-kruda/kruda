#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
export GOENV=off GOWORK=off GOFLAGS= GOEXPERIMENT= GOTOOLCHAIN=go1.26.11 CGO_ENABLED=0
export GOAMD64=v1

if [[ "$(uname -s)" != Linux ]]; then
    echo "Run this build on Linux to use Wing." >&2
    exit 2
fi

mode=${1:-release}
case "$mode" in
    release|experimental) ;;
    *) echo "usage: bash build.sh release|experimental" >&2; exit 2 ;;
esac

mkdir -p bin
cd fixture
go build -mod=readonly -trimpath -o ../bin/go-bench .
if [[ "$mode" == release ]]; then
    exit 0
fi

module_zip=$(go mod download -json github.com/go-kruda/kruda@v1.9.0 |
    python3 -c 'import json, sys; print(json.load(sys.stdin)["Zip"])')
cd ..
build_dir=$(mktemp -d "$PWD/bin/wing-src.XXXXXX")
unzip -q "$module_zip" -d "$build_dir"
module_dir="$build_dir/github.com/go-kruda/kruda@v1.9.0"
patch --batch --forward --fuzz=0 -d "$module_dir" -p1 < wing-256.patch
cp fixture/go.mod "$build_dir/wing.mod"
cp fixture/go.sum "$build_dir/wing.sum"
(
    cd fixture
    go mod edit -modfile "$build_dir/wing.mod" -replace "github.com/go-kruda/kruda=$module_dir"
    go build -mod=readonly -modfile "$build_dir/wing.mod" -trimpath -o ../bin/go-bench-size .
)
(cd fiber && go build -mod=readonly -trimpath -o ../bin/go-proxy .)
go build -trimpath -o bin/phase-load driver/main.go driver/load.go
(
    cd actix
    cargo_target=${CARGO_TARGET_DIR:-target}
    cargo +1.98.1 build --release --locked --target-dir "$cargo_target"
    cp "$cargo_target/release/actix-proxy" ../bin/actix-lab
)
