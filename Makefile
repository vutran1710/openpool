.PHONY: build op relay action-tool test coverage clean lint

build: op relay action-tool

op:
	go build -o bin/op ./cmd/op

relay:
	go build -o bin/relay ./cmd/relay

action-tool:
	go build -o bin/action-tool ./cmd/action-tool

test:
	OPENPOOL_HOME=$(shell mktemp -d) go test ./...

coverage:
	OPENPOOL_HOME=$(shell mktemp -d) go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...
