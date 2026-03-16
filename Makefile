.PHONY: build cli relay test coverage clean lint

build: cli relay

cli:
	go build -o bin/dating ./cmd/dating

relay:
	go build -o bin/relay ./cmd/relay

test:
	go test ./...

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...
