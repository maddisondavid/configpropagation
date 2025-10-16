.PHONY: all build test lint fmt tidy

MODULE := ./...

all: fmt lint build test

build:
	go build $(MODULE)

test:
	go test -cover $(MODULE)

lint:
	@golangci-lint run || (echo "Install golangci-lint or update .golangci.yml" && exit 1)

fmt:
	go fmt $(MODULE)

tidy:
	go mod tidy

