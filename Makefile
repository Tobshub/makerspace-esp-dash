.PHONY: dev infra api web test lint migrate seed simulator

dev: infra
	@echo "Infrastructure is up."
	@echo "Start the API:  make api"
	@echo "Start the UI:   make web"

infra:
	docker compose up -d

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
	@echo "Demo seed data arrives after teams, projects, and devices exist."

simulator:
	cd tools/device-simulator && go run . --help
