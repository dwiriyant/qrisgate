.PHONY: test test-cover lint tidy run-api migrate docker-up docker-hub-up docker-obs docker-down

test:
	go test ./...

test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -1

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0 run ./...

tidy:
	go mod tidy

run-api:
	go run ./cmd/api

migrate:
	goose -dir db/migrations postgres "$$DATABASE_URL" up

docker-up:
	docker compose up --build -d

docker-hub-up:
	docker compose -f docker-compose.hub.yml up -d

docker-obs:
	docker compose -f docker-compose.yml -f docker-compose.obs.yml up --build -d

docker-down:
	docker compose -f docker-compose.yml -f docker-compose.obs.yml down
