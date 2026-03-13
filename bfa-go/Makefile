.PHONY: build run mock test test-unit clean docker-build

build:
	go build -o bin/bfa-server ./cmd/server
	go build -o bin/mock-server ./cmd/mockserver

run:
	go run ./cmd/server

mock:
	go run ./cmd/mockserver

test:
	go test ./... -v -cover -race

test-unit:
	go test ./internal/... -v -cover

clean:
	rm -rf bin/

docker-build:
	docker build -t bfa-go .
