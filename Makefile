.PHONY: build cli relay indexer regcrypt test coverage clean lint

build: cli relay indexer regcrypt

cli:
	go build -o bin/dating ./cmd/dating

relay:
	go build -o bin/relay ./cmd/relay

indexer:
	go build -o bin/indexer ./cmd/indexer

regcrypt:
	go build -o bin/regcrypt ./cmd/regcrypt

test:
	DATING_HOME=$(shell mktemp -d) go test ./...

coverage:
	DATING_HOME=$(shell mktemp -d) go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...
