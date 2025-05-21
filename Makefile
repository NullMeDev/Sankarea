.PHONY: build test migrate up down deploy

include .env
export

build:
	docker-compose build sankarea
	go build -o bin/sankarea cmd/sankarea/main.go

test:
	go test ./...
	docker-compose run --rm sankarea migrate -path ./migrations -database "$$DB_DSN" version

migrate:
	docker-compose run --rm sankarea migrate -path ./migrations -database "$$DB_DSN" up

up:
	docker-compose up -d

down:
	docker-compose down

deploy: build migrate up
	echo "Deployed on $(date)"
