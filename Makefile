.PHONY: dev infra api web test lint migrate seed simulator worker

dev: infra
	@echo "Infrastructure is up."
	@echo "Start the API:  make api"
	@echo "Start the UI:   make web"
	@echo "Full stack:     docker compose up -d --build  (UI http://localhost:8088)"

infra:
	docker compose up -d postgres mosquitto redis

api:
	cd backend && go run ./cmd/api

web:
	cd frontend && npm run dev

test:
	cd backend && go test ./...
	cd tools/device-simulator && go test ./...
	cd frontend && npm run build

lint:
	cd backend && go vet ./...
	cd tools/device-simulator && go vet ./...
	cd frontend && npm run lint

migrate:
	cd backend && go run ./cmd/migrate

seed:
	cd backend && go run ./cmd/seed

simulator:
	cd tools/device-simulator && go run . --help

worker:
	cd backend && go run ./cmd/worker
