.PHONY: build test bench vet lint fmt clean

build:
	go build ./...

test:
	go test ./...

bench:
	go test -bench=. -benchmem ./...

vet:
	go vet ./...

lint: vet
	gofmt -l .

fmt:
	gofmt -w .

clean:
	go clean ./...
