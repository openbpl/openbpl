BINARY := openbpl
MODULE := github.com/openbpl/openbpl

.PHONY: build run clean test lint vet fmt tidy setup

setup:
	go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5700.1 install --with-deps chromium

build:
	@mkdir -p bin
	go build -o bin/$(BINARY) ./cmd

run: build
	./bin/$(BINARY)

clean:
	rm -rf bin

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint: vet fmt

tidy:
	go mod tidy
