.PHONY: build cli relay test clean lint

build: cli relay

cli:
	go build -o bin/dating ./cmd/dating

relay:
	go build -o bin/relay ./cmd/relay

test:
	go test ./...

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...
