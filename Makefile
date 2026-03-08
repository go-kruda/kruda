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
	go clean ./...
