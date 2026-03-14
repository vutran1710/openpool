.PHONY: build cli server test clean db-start db-stop db-reset

build: cli server

cli:
	go build -o bin/dating ./cmd/dating

server:
	go build -o bin/server ./cmd/server

test:
	go test ./...

clean:
	rm -rf bin/

db-start:
	npx supabase start

db-stop:
	npx supabase stop

db-reset:
	npx supabase db reset

lint:
	golangci-lint run ./...

run-server:
	go run ./cmd/server

run-cli:
	go run ./cmd/dating
