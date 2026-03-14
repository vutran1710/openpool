.PHONY: build test clean lint

build:
	go build -o bin/dating ./cmd/dating

test:
	go test ./...

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...

run:
	go run ./cmd/dating
