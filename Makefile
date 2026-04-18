.PHONY: build test bench vet lint fmt clean

build:
	go build ./...

test:
	go test -tags kruda_stdjson ./...

bench:
	go test -bench=. -benchmem -tags kruda_stdjson ./...

vet:
	go vet ./...

lint: vet
	gofmt -l .

fmt:
	gofmt -w .

clean:
	@go clean ./...
	@rm -f *.test *.out *.prof
	@rm -f auth auto-crud crud database di-services health-checks hello json-api sse static-files testing typed-handlers di-comparison
	@find examples -maxdepth 2 -type f -perm -u+x ! -name '*.sh' ! -name '*.go' ! -name '*.md' -delete 2>/dev/null || true
	@echo "clean."
