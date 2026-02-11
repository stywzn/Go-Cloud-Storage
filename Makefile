.PHONY: run build docker-up

run:
	go run cmd/server/main.go

build:
	go build -o bin/storage-server cmd/server/main.go

docker-up:
	docker compose up -d