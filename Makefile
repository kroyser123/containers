.PHONY: build run test vet up down

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

up:
	docker compose up --build -d

down:
	docker compose down
