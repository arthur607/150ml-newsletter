.PHONY: up down migrate migrate-down test test-go generate

up:
	docker compose up -d

down:
	docker compose down

migrate:
	goose -dir packages/database/migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir packages/database/migrations postgres "$(DATABASE_URL)" down

generate:
	go generate ./...

test:
	docker compose up -d
	sleep 2
	DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
	  go test ./services/ingestion/... ./services/processing/... ./services/api/... -v
	cd services/ml && python -m pytest -v

test-go:
	DATABASE_URL=postgres://curator:curator@localhost:5432/curator?sslmode=disable \
	  go test ./services/ingestion/... ./services/processing/... ./services/api/... -v
