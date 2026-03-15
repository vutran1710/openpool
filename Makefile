.PHONY: build cli relay seedpool test clean lint

build: cli relay

cli:
	go build -o bin/dating ./cmd/dating

relay:
	go build -o bin/relay ./cmd/relay

seedpool:
	go run ./cmd/seedpool -out ../dating-test-pool -registry-out ../dating-test-registry/pools/test-pool

test:
	go test ./...

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...
